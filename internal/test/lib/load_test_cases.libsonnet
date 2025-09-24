local flattenTests(value) =
  if std.objectHasAll(value, 'inputs') then
    [value]
  else if std.objectHas(value, 'tests') then
    [
      test {
        name: '%s/%s' % [std.get(value, 'name', ''), std.get(test, 'name', '')],
      }
      for test in std.flattenArrays([flattenTests(test) for test in value.tests])
    ]
  else [];

local loadTestCase(testConfig) = flattenTests(testConfig);

loadTestCase
