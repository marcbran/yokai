local h = import './html.libsonnet';
local lib = import './lib.libsonnet';

local view(config, key, model, fragment) =
  local app = lib.extractFromObject(config, key);
  local view = std.get(app.app, 'view', function(model) '');
  local elem = view(model);
  elem;

view
