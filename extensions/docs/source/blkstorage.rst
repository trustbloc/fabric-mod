
Block Storage Enhancements
==========================

Block Storage was originally implemented with a FileSystem storage using
indexes stored in LevelDB. This filesystem storage is not scallable and
requires a separate storage for the indexes. The new storage used in this
mod project is CouchDB

CouchDB
-------

There are several benefits for using CouchDB storage for blocks, among these
are:
   - The use of a distributed and scalable storage between peers
   - Indexes are managed in the same storage

.. Licensed under the Apache License, Version 2.0 (Apache-2.0)
https://www.apache.org/licenses/LICENSE-2.0
