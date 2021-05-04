<a name="v0.6.0"></a>
# [v0.6.0](https://github.com/qri-io/qfs/compare/v0.5.0...v) (2021-05-04)

This latests release adds a new important `WriteWithHooks` function that runs a hook for each file written to the filesystem, rolling back any writes if there are errors. This opens the door for progress updates, as well as adjusting subsequent files on the fly based on previous writes.

We've also added a new `DisableBootstrap` option to the qfs config, that allows you to run your node without bootstrapping to the network, as well as adding support for "PUT" over ipfs http.

Finally, we've fixed an important bug in linux distros, that allows copying the underlying filesystem on a cross-linked device.

### Bug Fixes

* **adder:** failed adds don't remove blocks because we don't have 'soft delete' ([20e9e18](https://github.com/qri-io/qfs/commit/20e9e18))
* **linux:** migration fix for copying on cross link device ([8bdb020](https://github.com/qri-io/qfs/commit/8bdb020))


### Features

* **cafs:** MerkelizeHooks to modify DAG persistence mid-flight ([164aadc](https://github.com/qri-io/qfs/commit/164aadc))
* **CAFS:** repurpose CAFS acronym as a filesystem property ([0e309e0](https://github.com/qri-io/qfs/commit/0e309e0))
* **ipfs_http:** support PUT on IPFS over http ([b257edd](https://github.com/qri-io/qfs/commit/b257edd))
* **mux:** add KnownFSTypes to list filesystem prefixes ([f5c12e4](https://github.com/qri-io/qfs/commit/f5c12e4))
* **qipfs:** add `DisableBootstrap` as qipfs config option ([75277f1](https://github.com/qri-io/qfs/commit/75277f1))
* **qipfs:** use rabin chunker ([e0a6727](https://github.com/qri-io/qfs/commit/e0a6727))



<a name="v0.5.0"></a>
# [v0.5.0](https://github.com/qri-io/qfs/compare/v0.4.2...v0.5.0) (2020-06-29)

This release is jumping to v0.5.0 to skip a bad version (v0.4.2 was publishe in error, don't use!)

v0.5.0 overhauls a bunch of APIS and introduces all sorts of breaking changes.


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



