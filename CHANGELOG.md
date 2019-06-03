<a name="0.1.0"></a>
#  (2019-06-03)

This is the first proper release of `qfs`. In preparation for go 1.13, in which `go.mod` files and go modules are the primary way to handle go dependencies, we are going to do an official release of all our modules. This will be version v0.1.0 of `qfs`. We'll be working on adding details & documentation in the near future.

### Bug Fixes

* **adder:** because we now have VisConfig as a possible field in dataset, we need to make another channel available to the adder (from 8 to 9). Fixed in both ipfs and memfs ([aef9d08](https://github.com/qri-io/qfs/commit/aef9d08))
* **Fetcher:** fix fetcher interface ([91fd46c](https://github.com/qri-io/qfs/commit/91fd46c))
* **fsrepo:** close fsrepo on context cancellation ([ff18631](https://github.com/qri-io/qfs/commit/ff18631))
* **ipfs:** coreapi not being reallocated when going online ([7394a3c](https://github.com/qri-io/qfs/commit/7394a3c))
* **ipfs plugins:** export LoadPlugins call once for package tests ([e4205f8](https://github.com/qri-io/qfs/commit/e4205f8))


### Code Refactoring

* **cafs.Filestore:** work with strings instead of datastore.Key ([8f5ce11](https://github.com/qri-io/qfs/commit/8f5ce11))


### Features

* **cache, gcloud:** new cache interface, starting into gcloud cache ([1ed3e3a](https://github.com/qri-io/qfs/commit/1ed3e3a))
* **destroyer:** add destroyer interface ([43a6e0f](https://github.com/qri-io/qfs/commit/43a6e0f))
* **Filestore:** added PathPrefix to filestore interface ([a79eb74](https://github.com/qri-io/qfs/commit/a79eb74))
* **GoOnline:** added new methods to take offline node online ([67ca85e](https://github.com/qri-io/qfs/commit/67ca85e))
* **IPFS api:** add options to enable IPFS API, pubsub ([ed80762](https://github.com/qri-io/qfs/commit/ed80762))
* **mapstore:** Improve Mapstore for better tests. ([3ebe881](https://github.com/qri-io/qfs/commit/3ebe881))
* **mapstore.Print:** new method to print mapstore contents ([4055c2f](https://github.com/qri-io/qfs/commit/4055c2f))
* **MemFS:** make MemFS actually do something ([840dd74](https://github.com/qri-io/qfs/commit/840dd74))
* **migration error:** detect ipfs migration errors ([81556ca](https://github.com/qri-io/qfs/commit/81556ca))
* bring file interface into package ([3eae520](https://github.com/qri-io/qfs/commit/3eae520))


### BREAKING CHANGES

* **cafs.Filestore:** cafs.Filestore interface methods accept and return string values instead of datastore.Key.



