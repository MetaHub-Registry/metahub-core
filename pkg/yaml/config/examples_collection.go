package config

func FTyamlCollectionGeneric() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "generic",
	}
}

func FTyamlCollectionZen3() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "zen3",
	}
}

func FTyamlCollectionArm64() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "arm64",
	}
}

func FTyamlCollectionamd64() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "amd64",
	}
}

func FTyamlCollectionCycloneDX() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "cyclonedx",
	}
}
