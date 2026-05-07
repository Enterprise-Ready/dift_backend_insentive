package config

type FeatureFlags struct {

	// =====================
	// Core Infrastructure
	// =====================

	EnableDatabase bool
	EnableHTTP     bool
	EnableGRPC     bool

	// =====================
	// Business Modules
	// =====================

	EnableAdmin      bool
	EnableClaim      bool
	EnableCalculator bool
	EnableRewardFlow bool

	// =====================
	// Messaging - Producer
	// =====================

	EnableRedpandaProducer bool
	EnableNATSProducer     bool

	// =====================
	// Messaging - Consumer
	// =====================

	EnableRedpandaConsumer bool
	EnableNATSConsumer     bool
}

func LoadFeatureFlags() FeatureFlags {

	return FeatureFlags{

		// =====================
		// Core
		// =====================

		EnableDatabase: true,
		EnableHTTP:     true,
		EnableGRPC:     true,

		// =====================
		// Business
		// =====================

		EnableAdmin:      true,
		EnableClaim:      true,
		EnableCalculator: true,
		EnableRewardFlow: true,

		// =====================
		// Messaging Producer
		// =====================

		EnableRedpandaProducer: false,
		EnableNATSProducer:     true,

		// =====================
		// Messaging Consumer
		// =====================

		EnableRedpandaConsumer: false,
		EnableNATSConsumer:     false,
	}
}
