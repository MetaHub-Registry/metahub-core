package types

const ()

var (
	RuleAnnoZen3 = Rule{
		Name:  "target",
		Tag:   "platform",
		Type:  "anno",
		Label: "target:platform",
		Op:    "eq",
		Key:   "org.supercontainers.hardware.cpu.target.microarch",
		Value: "zen3",
	}
	RuleAnnoZen2 = Rule{
		Name:  "target",
		Tag:   "platform",
		Type:  "anno",
		Label: "target:platform",
		Op:    "eq",
		Key:   "org.supercontainers.hardware.cpu.target.microarch",
		Value: "zen2",
	}
	// Platform
	RulePlatLinux = Rule{
		Name:  "os",
		Tag:   "linux",
		Type:  "plat",
		Op:    "eq",
		Key:   "os",
		Value: "linux",
	}
)
