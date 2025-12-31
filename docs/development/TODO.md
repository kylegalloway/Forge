# TODO

## Future Enhancements (Long-term)

### Medium Value

* **Additional destination types** - Artifactory and Nexus registry support (currently supports OCI, S3, Local)
* **Multi-architecture bundle builds** - Cross-platform builds for arm64/amd64 in single job execution
* **Enhanced resource scheduling** - Node affinity, tolerations, and priority classes configurable per-job

### Low Priority (Polish)

* **Webhook TLS certificate rotation** - Automated cert-manager integration for seamless certificate rotation without webhook downtime
* **Structured event streaming** - CloudEvents format for integration with external systems
* **Cost attribution labels** - Automatic tagging of Jobs/Pods with cost center metadata for chargeback
