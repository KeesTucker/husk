package services

type ServiceName string

const (
	ServiceDriver ServiceName = "driver"
	ServiceECU    ServiceName = "ecu"
	ServiceGUI    ServiceName = "gui"
	ServiceLogger ServiceName = "logger"
)

var registry = make(map[ServiceName]interface{})

// Register a service by name
func Register(name ServiceName, service interface{}) {
	registry[name] = service
}

// Get retrieves a registered service
func Get(name ServiceName) interface{} {
	service := registry[name]
	return service
}
