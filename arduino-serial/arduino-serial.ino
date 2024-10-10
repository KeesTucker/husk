#include <Arduino.h>
#include <mcp2515.h>

const byte START_MARKER = 0x7E;
const byte END_MARKER = 0x7F;
const byte ESCAPE_CHAR = 0x1B;

struct can_frame frame;

const unsigned long TIMEOUT_MS = 100; // Timeout for waiting on Serial

void setup() {
    Serial.begin(115200);
}

void loop() {
    if (Serial.available() > 0) {
        if (readCanBusFrame(frame)) {
            // Validate the frame
            if (frame.can_dlc <= 8) {
                // Send the CAN frame back for testing purposes
                randomSeed(analogRead(0));
                uint16_t canId = random(0, 2048);
                frame.can_id = canId;
                sendCanBusFrame(frame);
            }
        }
    }
}

bool readByteWithTimeout(byte &result, unsigned long timeoutMs = TIMEOUT_MS) {
    unsigned long startTime = millis();
    while (millis() - startTime < timeoutMs) {
        if (Serial.available() > 0) {
            result = Serial.read();
            return true;
        }
    }
    return false; // Timed out
}

bool readCanBusFrame(struct can_frame &frame) {
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
            return false; // Timeout while waiting for byte
        }

        if (incomingByte == END_MARKER) {
            // End marker found, break loop
            break;
        } else if (incomingByte == ESCAPE_CHAR) {
            // Escape character, read the next byte
            if (!readByteWithTimeout(incomingByte)) {
                return false; // Timeout while waiting for next byte
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
                    return false; // Invalid escape sequence
            }
        } else {
            unstuffed[index++] = incomingByte;
        }

        // Prevent buffer overflow
        if (index >= sizeof(unstuffed)) {
            return false; // Frame too large
        }
    }

    // Parse unstuffed data
    if (index < 4) {
        return false;
    }

    // CAN ID (2 bytes) - matching Go's decoding
    frame.can_id = ((uint16_t)unstuffed[0] << 8) | unstuffed[1];

    // DLC
    frame.can_dlc = unstuffed[2];
    if (frame.can_dlc > 8) {
        return false;
    }

    // Data (up to DLC bytes)
    for (int i = 0; i < frame.can_dlc; i++) {
        frame.data[i] = unstuffed[3 + i];
    }

    // Checksum
    byte receivedChecksum = unstuffed[3 + frame.can_dlc];
    byte calculatedChecksum = calculateCRC8(frame.can_dlc, frame.data);

    // Verify checksum
    if (calculatedChecksum != receivedChecksum) {
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

void sendCanBusFrame(struct can_frame &frame) {
    // Start Marker
    Serial.write(START_MARKER);

    // CAN ID (2 bytes) - matching Go's encoding
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
    byte checksum = calculateCRC8(frame.can_dlc, frame.data);
    stuffByte(checksum);

    // End Marker
    Serial.write(END_MARKER);
}

byte calculateCRC8(size_t dlc, const byte *data) {
    byte crc = 0x00;
    const byte polynomial = 0x07; // CRC-8-CCITT

    for (size_t i = 0; i < dlc; i++) {
        crc ^= data[i];
        for (byte j = 0; j < 8; j++) {
            if (crc & 0x80) {
                crc = (crc << 1) ^ polynomial;
            } else {
                crc <<= 1;
            }
        }
    }

    return crc;
}
