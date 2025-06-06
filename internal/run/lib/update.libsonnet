local lib = import './lib.libsonnet';

local update(config, key, topic, payload, model) =
  local app = lib.extractFromObject(config, key);
  app.app.update[topic](model, payload);

update
