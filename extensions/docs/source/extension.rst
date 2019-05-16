Hyperledger Fabric Extensions
=============================

Hyperledger Fabric Extensions are a set of enhancements to Hyperledger Fabric.
The aim of these enhancements are to allow for performance enhancements on a
peer using several mechanisms such as DCAS (Distributed Content Addressable
Storage) for private data collections, in-memory data cache stores instead of
leveldb or fs stores, peer roles separations (committer vs endorser), etc.

Hyperledger Fabric Extensions are integrated into Hyperledger Fabric through
hooks. The following extensions packages are currently available:

.. toctree::
   :maxdepth: 1

   blkstorage
   collections
   gossip
   idstore
   pvtdatastorage
   roles
   service
   transientstore
