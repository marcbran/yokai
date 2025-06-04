local flattenObject(value, separator='/', anchorField='app') =
  if std.type(value) == 'object' && !std.objectHas(value, anchorField) then
    std.foldl(function(acc, curr) acc + curr, [
      {
        [std.join(separator, std.filter(function(key) key != '', [child.key, childChild.key]))]: childChild.value
        for childChild in std.objectKeysValues(flattenObject(child.value, separator, anchorField))
      }
      for child in std.objectKeysValues(value)
    ], {})
  else if std.type(value) == 'object' then
    { '': value }
  else if std.type(value) == 'array' then
    std.foldl(function(acc, curr) acc + curr, [
      {
        [std.join(separator, std.filter(function(key) key != '', [child.key, childChild.key]))]: childChild.value
        for childChild in std.objectKeysValues(flattenObject(child.value, separator, anchorField))
      }
      for child in std.mapWithIndex(function(index, value) { key: index, value: value }, value)
    ], {})
  else {};

local listApps(config) =
  std.mapWithKey(function(key, app) {
    init: std.get(app.app, 'init', null),
    subscriptions: std.get(app.app, 'subscriptions', []),
  }, flattenObject(config));

listApps
