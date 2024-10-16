package processors

import "husk/frames"

type ECUProcessor interface {
	TesterPresentLoop()
	ProcessFrame(frame *frames.Frame)
}
