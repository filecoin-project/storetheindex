# Purpose of this Doc

This doc should cover our discussions on how to scale `storetheindex`. It should be used as a reference and starting point for further discussions on scaling (to avoid rehashing the same arguments). It includes high level ideas on how to scale read and write paths, but does not prescribe anything specific to leave flexibility for the actual implementation.


# Scaling

When we talk about scaling we are talking about scaling two things:

1.  The read path.
    This is where a client will ask us who can provide a given CID.
2.  The write path.
    This is where a storage provider tells us that they can provide a CID.


# Read Scaling Strategies


## Full Duplicate

A full duplicate can double the read throughput since any machine can equally handle the request.


## Shard by lookup key (CID)

Our data is evenly distributed across the key space thanks to that property of our hashing function. We should take advantage of that by creating database shards that only manage some portion of that keyspace.


### Example

Say that we have 256 CIDs that we&rsquo;ve indexed. The CIDs have a property that half of them will have a hash less than `0x888..` and the other half will have a hash greater than `0x888...`.

We could have two database shards that each manage half the keyspace. When a query comes in we map it to the shard that manages that keyspace.


### Scaling up

When we want to increase read capacity we can add another shard to the set. The new shard will now manage a portion of that keyspace and do an initial spin up by asking the other relevant shards for data in its keyspace.


# Write Scaling Strategies


## Vertical scaling

Beefier instances.


## Code optimization

I think there&rsquo;s still room to improve ingest capactiy (write capacity) on the internal side as well


## Sharded write router (alongside sharding by lookup key)

This is a new service that reads from a subset of storage providers, unpacks the entries, and splits the entries up by the read shard key. Then forwards these entries to the correct database shards.

This way a database shard itself doesn&rsquo;t have to listen to every storage provider.

This router can be scaled by sharding by the provider&rsquo;s peer id.


# Anti-patterns


## Fanout queries (a.k.a Scatter Gather query)

A fanout query is a query that needs to query (or &ldquo;fan out&rdquo; to) multiple databases before it can return an answer. A fanout query is not ideal because it requires all database shards to return an answer before the final answer can be returned. It also suffers from the availability being the product of all the shards (e.g. if you have 2 shards and each have 50% uptime, the final result will be 25% uptime). The response latency is tied to the slowest machine.

