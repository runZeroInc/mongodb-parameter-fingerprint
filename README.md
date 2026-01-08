# mongodb-parameter-fingerprint

A tool to fingerprint MongoDB versions using the `listCommands` output and specifically the help text from the `setParameter` command.

This is useful for identifying MongoDB instances where `buildInfo` is no longer exposed pre-authentication (8.2.1+).

For example, version `8.2.1` can be matched using hash `d98d6221a22a37dce7f4fffc05ec05a9`.
The following two versions (`8.2.2`-`8.2.3`) can be matching using hash `2d7cb04d67cc9291f3bb561f32b7feaa`.
Notably `8.2.2` introduced the following changes:
```json
{
  "2d7cb04d67cc9291f3bb561f32b7feaa": {
    "version_min": "8.2.2",
    "version_max": "8.2.3",
    "versions": [
      "8.2.2",
      "8.2.3"
    ],
    "version_prev": "8.2.1",
    "params_added": [
      "ingressRequestRateLimiterApplicationExemptions",
      "internalQueryPermitMatchSwappingForComplexRenames",
      "internalReduceAccumulatedValueDepthCheckInterval",
      "minimalWriteConflictRetryCountForStateDump",
      "oplogSamplingAsyncYieldIntervalMs",
      "proxyProtocolMaximumPendingConnections",
      "proxyProtocolMaximumWaitBackoffMillis",
      "proxyProtocolTimeoutSecs",
      "useSlowCollectionTruncateMarkerScanning",
      "writeConflictRetryCountForDumpState"
    ],
    "count": 792
  }
}
```


## Instructions

1. Configure docker or podman on your development system.
2. Install a recent version of Go (1.25+)/
3. Run `./update-from-docker.sh` to extract fingerprints from newer MongoDB containers.
4. Use `data/matches.json` for matching `setParameter` help output (MD5) to versions.
5. Be amazed at how many parameters are changed between minor versions of MongoDB.

## Other uses

The help output files in the `data` directory can be used to identify unauthenticated
commands by version and generally review the evolution of MongoDB features. 

```sh
$ grep -L '"requiresAuth": false' data/*/buildInfo.json
data/8.2.1/buildInfo.json
data/8.2.2/buildInfo.json
data/8.2.3/buildInfo.json
```

Some interesting highlights:

- The big one: `buildInfo` is restricted as of `8.2.1`.
- The `waitForFailPoint` command was removed after `8.0.9`.
- The `whatsmyuri` command was removed after `7.0.9`.