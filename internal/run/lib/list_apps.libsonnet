local lib = import './lib.libsonnet';

local listApps(config) =
  std.mapWithKey(function(key, app) {
    init: std.get(app.app, 'init', null),
    subscriptions: std.get(app.app, 'subscriptions', []),
  }, lib.flattenObject(config));

listApps
