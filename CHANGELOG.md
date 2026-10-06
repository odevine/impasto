# Changelog

## [0.2.1](https://github.com/odevine/impasto/compare/v0.2.0...v0.2.1) (2026-10-06)


### Performance Improvements

* **raster:** encode to 8-bit through a table and ingest RGBA directly ([2ffd980](https://github.com/odevine/impasto/commit/2ffd980daab382f840965a2b50181444fb95edc1))

## [0.2.0](https://github.com/odevine/impasto/compare/v0.1.1...v0.2.0) (2026-10-06)


### Features

* **canvas:** let a layer cover only part of the document ([#10](https://github.com/odevine/impasto/issues/10)) ([986550a](https://github.com/odevine/impasto/commit/986550a873ee0acc254a6bbb79325e3584319b6e))
* **canvas:** let a layer supply its content when it is composited ([#11](https://github.com/odevine/impasto/issues/11)) ([25c1bfb](https://github.com/odevine/impasto/commit/25c1bfb3cd2a40286b5eae80a25827bad9146ffb))


### Performance Improvements

* **canvas:** skip layer clones and clip bases that nothing reads ([#8](https://github.com/odevine/impasto/issues/8)) ([7924d01](https://github.com/odevine/impasto/commit/7924d01e672fa167aedc7fcde652bc98defff17a))


### Miscellaneous Chores

* release 0.2.0 ([69681f1](https://github.com/odevine/impasto/commit/69681f129b01f5caf0208b4b3b7cbf0dc343563d))

## [0.1.1](https://github.com/odevine/impasto/compare/v0.1.0...v0.1.1) (2026-09-18)


### Documentation

* cite the blend specs and neutralize comment tone ([d79be30](https://github.com/odevine/impasto/commit/d79be304db90957f44540cddebb804c3c7d71357))

## 0.1.0 (2026-09-18)


### Features

* implement pure-Go 2D layer compositing library ([286c211](https://github.com/odevine/impasto/commit/286c211fec20f49fa13f89a11d73c7982445846d))
