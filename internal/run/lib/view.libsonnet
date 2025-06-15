local lib = import './lib.libsonnet';
local h = import 'html/main.libsonnet';

local view(config, key, model) =
  local app = lib.extractFromObject(config, key);
  local view = std.get(app.app, 'view', function(model) '');
  local elem = view(model);
  std.manifestXmlJsonml(
    h.html({}, [
      h.head({}, [
        h.script({ src: 'https://unpkg.com/htmx.org@2.0.4', integrity: 'sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+', crossorigin: 'anonymous' }),
        h.script({ src: 'https://unpkg.com/htmx-ext-ws@2.0.2', integrity: 'sha384-932iIqjARv+Gy0+r6RTGrfCkCKS5MsF539Iqf6Vt8L4YmbnnWI2DSFoMD90bvXd0', crossorigin: 'anonymous' }),
      ]),
      h.body({}, [
        h.div({ 'hx-ext': 'ws', 'ws-connect': '/%(key)s' % { key: key } }, [
          h.div({ id: key }, [
            elem,
          ]),
        ]),
      ]),
    ]),
  );

view
