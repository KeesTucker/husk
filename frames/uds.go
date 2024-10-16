package frames

func GetUDSTesterPresentFrame() *Frame {
	return &Frame{
		DLC:  0x01,
		Data: [8]uint8{0x3E},
	}
}
