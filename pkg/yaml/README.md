# YAML pkg

This `package` reads in MetaHub configuration (e.g. via the GitLab MetaHub Community Edition).
The goal is to 

1. iterate through all manifest and make sure they are cached.
    a. tag -> digest resolution
    b. manifest.digest
    c. imgConfig.digest
    d. layer
2. Once all data is cached, insert Collection into GraphDB. This process needs to be idempotent.

