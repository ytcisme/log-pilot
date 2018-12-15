<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Build Custom Filebeat Image](#build-custom-filebeat-image)
  - [Elasticsearch Mapping](#elasticsearch-mapping)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# Build Custom Filebeat Image

## Elasticsearch Mapping

Filebeat loads a json file and send request to ES to create a [template](https://www.elastic.co/guide/en/elasticsearch/reference/current/mapping.html#_updating_existing_field_mappings).


``` json
  {
      "template": "logstash-*",
      "settings": {
          "index.refresh_interval": "30s", (1)
          "index.number_of_replicas": 1, (2)
          "index.number_of_shards": 3
      },
      "mappings": {
          "_default_": {
              "_all": {
                  "enabled": false
              },
              "dynamic_templates": [
                  {
                      "strings_as_keywords": {
                          "match_mapping_type": "string",
                          "unmatch": "log",
                          "mapping": {
                              "type": "keyword"
                          }
                      }
                  },
                  {
                      "log": {
                          "match": "log",
                          "match_mapping_type": "string",
                          "mapping": {
                              "type": "text",
                              "analyzer": "standard",
                              "norms": false
                          }
                      }
                  }
              ]
          }
      }
  }
```

- (1) Increasing this value will [allow larger segments to flush and decreases future merge pressure](https://www.elastic.co/guide/en/elasticsearch/reference/6.4/tune-for-indexing-speed.html#_increase_the_refresh_interval ).

- (2) An [index](https://www.elastic.co/guide/en/elasticsearch/reference/6.4/_basic_concepts.html#getting-started-shards-and-replicas) is subdivided into `number_of_shards` shards. Every shard have `number_of_replicas` replica. If a shard have 1 replica, then there will be a primary shard and a replica shard.
For a 3 shards, 1 replicas, 3 node es cluster,  an index foo is possibly stored as following

| Node1 | Node2 | Node3|
| -- | -- | -- |
| foo_shard1_primary, foo_shard2_primary | foo_shard1_replica, foo_shard3_primary | foo_shard2_replica, foo_shard3_replica |