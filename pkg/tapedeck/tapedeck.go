package tapedeck

import (
	"fmt"
	"strings"
)

// adapterRegistry holds registered adapters by station call sign.
var adapterRegistry = make(map[string]func() Adapter)

// RegisterAdapter registers an adapter factory for a station call sign.
func RegisterAdapter(callSign string, factory func() Adapter) {
	adapterRegistry[strings.ToUpper(callSign)] = factory
}

// GetAdapter returns an adapter for the given station call sign.
func GetAdapter(callSign string) (Adapter, error) {
	factory, ok := adapterRegistry[strings.ToUpper(callSign)]
	if !ok {
		return nil, fmt.Errorf("unknown station: %s", callSign)
	}
	return factory(), nil
}

// ListStations returns a list of registered station call signs.
func ListStations() []string {
	stations := make([]string, 0, len(adapterRegistry))
	for callSign := range adapterRegistry {
		stations = append(stations, callSign)
	}
	return stations
}
