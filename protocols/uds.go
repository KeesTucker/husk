package protocols

func GetUDSTesterPresentFrame() *CanFrame {
	return &CanFrame{
		DLC:  0x01,
		Data: [8]uint8{0x3E},
	}
}

// TODO: provide send frame wrapper here called sendudsframe that should be called when sending uds frames
