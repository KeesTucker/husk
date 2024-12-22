package seedkey

type SecurityLevel int8

const (
	SecurityLevelUnspecified SecurityLevel = iota
	SecurityLevel1
	SecurityLevel2
	SecurityLevel3
)
