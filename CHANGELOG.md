# Changelog

## [1.0.3](https://github.com/KeesTucker/husk/compare/husk-v1.0.2...husk-v1.0.3) (2024-10-11)


### Bug Fixes

* read timeout should be 1 ([f1e04fd](https://github.com/KeesTucker/husk/commit/f1e04fd5f47fff6b7ea187ec1471fc72eeaabd25))

## [1.0.2](https://github.com/KeesTucker/husk/compare/husk-v1.0.1...husk-v1.0.2) (2024-10-11)


### Bug Fixes

* reference logo in fyne cross compile command ([4d545c0](https://github.com/KeesTucker/husk/commit/4d545c04bcb13c56710931b728c0d8036aac62d8))

## [1.0.1](https://github.com/KeesTucker/husk/compare/husk-v1.0.0...husk-v1.0.1) (2024-10-11)


### Bug Fixes

* increased read timeout to 5 milliseconds ([1050f11](https://github.com/KeesTucker/husk/commit/1050f11f0c8127e730612a3a066bef2fc8f41187))

## 1.0.0 (2024-10-11)


### Features

* add proper .gitignore ([282d7cb](https://github.com/KeesTucker/husk/commit/282d7cbc71bca7a0558e6abf2419fa9bccb8fa7c))
* initial commit ([61a9b4a](https://github.com/KeesTucker/husk/commit/61a9b4a1e5b62c2da79d96487997083aab3cbf1b))
* much more reliable serial communication, includes crc, better data structure, start and end deliminators, byte stuffing, read write blocking etc. added arduino serial ino. todo: needs to ack messages and receive acks, also needs to gracefully close serial ports and running go routines when app is closed. ([a227eda](https://github.com/KeesTucker/husk/commit/a227eda6ecda8b540dda5329506308ca3a89e58b))
* removed overcomplicated channel system for read and write. now we have a read timeout and a single mutex. also added acking. removed bufio and just rely on serial port read. ([94803e6](https://github.com/KeesTucker/husk/commit/94803e63ca5f8e93245e38ce3237c04a92c7443d))
* updated tests ([3857ca7](https://github.com/KeesTucker/husk/commit/3857ca7bb2c2fee007d84c91bb7aff4a0c2e2ee7))
* use bufio.Reader for reading serial instead of doing it manually. convert to 8 byte can frame ([55238b9](https://github.com/KeesTucker/husk/commit/55238b907cd33bae04f0292cf25ffb5372921e66))
* vastly improved serial comms, added tests for arduino driver! ([33ca103](https://github.com/KeesTucker/husk/commit/33ca1035ff1dbc3b0e8913bccd029c0f2ce2eff2))


### Bug Fixes

* removed percentage based autoscroll ([9cc2a39](https://github.com/KeesTucker/husk/commit/9cc2a39af0e1665c655d84cd2f0ff86d64601dc9))
