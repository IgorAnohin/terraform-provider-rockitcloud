package services

type prometheusManager struct {
	service
}

var Prometheus = prometheusManager{
	service{
		name:               ServiceTypePrometheus,
		class:              []string{ServiceClassMonitoring},
		defaultClass:       ServiceClassMonitoring,
		allowArbitrator:    false,
		allowBackup:        false,
		dataVolumeRequired: true,
		usersEnabled:       false,
		databasesEnabled:   false,
		loggingEnabled:     false,
		monitoringEnabled:  false,
	},
}
