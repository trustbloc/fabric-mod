Transient Private Data Store Enhancements
=========================================

The Transient Private Data Store enhancements allows for faster (local)
transient storage on the peer to enhance transactions endorsements and
eventually commits. The original storage used by the peer is leveldb which
is limited by the local filesystem I/O operations.

GCache
------

The LevelDB storage implementation of the Transient store has been replaced
by GCache to store private data in-memory as opposed to on disk.

The same store functions are tested against the original Fabric transient
store unit tests and additional generic unit tests to ensure private data
consistency.

.. Licensed under the Apache License, Version 2.0 (Apache-2.0)
https://www.apache.org/licenses/LICENSE-2.0
