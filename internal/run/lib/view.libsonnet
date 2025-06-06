local lib = import './lib.libsonnet';

local view(config, key, model) =
  local app = lib.extractFromObject(config, key);
  local view = std.get(app.app, 'view', function(model) '');
  view(model);

view
