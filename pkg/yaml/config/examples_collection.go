package config

func FT_yamlCollection_generic() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "generic",
	}
}

func FT_yamlCollection_zen3() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "zen3",
	}
}

func FT_yamlCollection_arm64() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "arm64",
	}
}

func FT_yamlCollection_amd64() ManifestCollection {
	return ManifestCollection{
		Name: "qnib/featuretest",
		Tag:  "amd64",
	}
}
