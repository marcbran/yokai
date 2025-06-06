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

local extractFromObjectRec(value, key) =
  if std.length(key) == 0 then
    value
  else
    extractFromObjectRec(value[key[0]], key[1:]);

local extractFromObject(value, key, separator='/') =
  extractFromObjectRec(value, std.split(key, separator));

{
  flattenObject: flattenObject,
  extractFromObject: extractFromObject,
}
