#include <Arduino.h>
#include <mcp2515.h>

const byte START_MARKER = 0x7E;
const byte END_MARKER = 0x7F;
const byte ESCAPE_CHAR = 0x1B;
const byte ACK = 0x06;
const byte NACK = 0x15;

// MCP2515 CAN controller
MCP2515 mcp2515(9);  // CS pin 9

struct can_frame rxFrame;
struct can_frame txFrame;

const int MAX_RETRIES = 3; // Maximum number of retries
const unsigned long READ_TIMEOUT_MS = 5; // Timeout for waiting on Serial
const unsigned long ACK_TIMEOUT_MS = 100; // Timeout for waiting for ACK/NACK
const unsigned long RETRY_DELAY_MS = 100; // Timeout for waiting for ACK/NACK

void setup() {
    Serial.begin(921600);
}

void loop() {
    if (mcp2515.readMessage(&rxFrame) == MCP2515::ERROR_OK) {
      sendSerialCanBusFrame(rxFrame);
    }

    // Check for incoming CAN frames, read them
    if (Serial.available() > 0 && readSerialCanBusFrame(txFrame)) {
      // If we have an incoming frame relay it to the ECU
      if (mcp2515.sendMessage(&txFrame) == MCP2515::ERROR_OK) {
        // We managed to send the frame! ACK the serial read.
        Serial.write(ACK);
        return;
      }
      // We failed to send the frame! NACK the serial read.
      Serial.write(NACK);
    }
}

bool readByteWithTimeout(byte &result, unsigned long timeoutMs = READ_TIMEOUT_MS) {
    unsigned long startTime = millis();
    while (millis() - startTime < timeoutMs) {
        if (Serial.available() > 0) {
            result = Serial.read();
            return true;
        }
    }
    return false; // Timed out
}

bool readSerialCanBusFrame(struct can_frame &frame) {
    // Wait for the start marker
    byte startByte;
    if (!readByteWithTimeout(startByte) || startByte != START_MARKER) {
        return false; // Ignore unexpected bytes or timeout
    }

    // Buffer for storing unstuffed bytes
    byte unstuffed[24];
    int index = 0;

    // Read bytes until the end marker is found
    while (true) {
        byte incomingByte;
        if (!readByteWithTimeout(incomingByte)) {
            Serial.write(NACK); // Timeout while waiting for byte
            return false;
        }

        if (incomingByte == END_MARKER) {
            // End marker found, break loop
            break;
        } else if (incomingByte == ESCAPE_CHAR) {
            // Escape character, read the next byte
            if (!readByteWithTimeout(incomingByte)) {
                Serial.write(NACK); // Timeout while waiting for next byte
                return false;
            }
            switch (incomingByte) {
                case 0x01:
                    unstuffed[index++] = START_MARKER;
                    break;
                case 0x02:
                    unstuffed[index++] = END_MARKER;
                    break;
                case 0x03:
                    unstuffed[index++] = ESCAPE_CHAR;
                    break;
                default:
                    Serial.write(NACK); // Invalid escape sequence
                    return false;
            }
        } else {
            unstuffed[index++] = incomingByte;
        }

        // Prevent buffer overflow
        if (index >= sizeof(unstuffed)) {
            Serial.write(NACK); // Frame too large
            return false;
        }
    }

    // Parse unstuffed data
    if (index < 4) {
        Serial.write(NACK); // Incomplete frame
        return false;
    }

    // CAN ID (2 bytes)
    frame.can_id = ((uint16_t)unstuffed[0] << 8) | unstuffed[1];

    // DLC
    frame.can_dlc = unstuffed[2];
    if (frame.can_dlc > 8) {
        Serial.write(NACK); // Invalid DLC
        return false;
    }

    // Data (up to DLC bytes)
    for (int i = 0; i < frame.can_dlc; i++) {
        frame.data[i] = unstuffed[3 + i];
    }

    // Checksum
    byte receivedChecksum = unstuffed[3 + frame.can_dlc];
    byte calculatedChecksum = calculateCRC8(frame);

    // Verify checksum
    if (calculatedChecksum != receivedChecksum) {
        Serial.write(NACK); // Checksum mismatch
        return false;
    }

    return true;
}

void stuffByte(byte b) {
    switch (b) {
        case START_MARKER:
            Serial.write(ESCAPE_CHAR);
            Serial.write(0x01);
            break;
        case END_MARKER:
            Serial.write(ESCAPE_CHAR);
            Serial.write(0x02);
            break;
        case ESCAPE_CHAR:
            Serial.write(ESCAPE_CHAR);
            Serial.write(0x03);
            break;
        default:
            Serial.write(b);
    }
}

void sendSerialCanBusFrame(struct can_frame &frame) {
    int retries = 0;
    bool ackReceived = false;

    while (retries < MAX_RETRIES && !ackReceived) {
        // Send the frame
        sendFrameBytes(frame);

        // Wait for ACK or NACK
        ackReceived = waitForACK();

        if (!ackReceived) {
            // Retry after a delay
            delay(RETRY_DELAY_MS);
            retries++;
        }
    }

    if (!ackReceived) {
      // todo: error handling somehow
    }
}

void sendFrameBytes(struct can_frame &frame) {
    // Start Marker
    Serial.write(START_MARKER);

    // CAN ID (2 bytes)
    byte idHigh = (frame.can_id >> 8) & 0xFF;
    byte idLow = frame.can_id & 0xFF;
    stuffByte(idHigh);
    stuffByte(idLow);

    // DLC
    stuffByte(frame.can_dlc);

    // Data (only up to DLC)
    for (int i = 0; i < frame.can_dlc; i++) {
        stuffByte(frame.data[i]);
    }

    // Calculate and add checksum
    byte checksum = calculateCRC8(frame);
    stuffByte(checksum);

    // End Marker
    Serial.write(END_MARKER);
}

bool waitForACK() {
    unsigned long startTime = millis();
    while (millis() - startTime < ACK_TIMEOUT_MS) {
        if (Serial.available() > 0) {
            byte response = Serial.read();
            if (response == ACK) {
                return true;
            } else if (response == NACK) {
                return false;
            }
            // Ignore other bytes
        }
    }
    return false; // Timeout
}

byte calculateCRC8(const struct can_frame &frame) {
    byte crc = 0x00;

    // Include CAN ID (2 bytes)
    byte idBytes[2] = { (byte)(frame.can_id >> 8), (byte)(frame.can_id & 0xFF) };
    for (int i = 0; i < 2; i++) {
        crc = xorShift(crc, idBytes[i]);
    }

    // Include DLC
    crc = xorShift(crc, frame.can_dlc);

    // Include Data bytes
    for (size_t i = 0; i < frame.can_dlc; i++) {
        crc = xorShift(crc, frame.data[i]);
    }

    return crc;
}

byte xorShift(byte crc, byte b) {
    const byte polynomial = 0x07; // CRC-8-CCITT
    crc ^= b;
    for (byte i = 0; i < 8; i++) {
        if (crc & 0x80) {
            crc = (crc << 1) ^ polynomial;
        } else {
            crc <<= 1;
        }
    }
    return crc;
}

