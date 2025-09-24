local h = import './html.libsonnet';

local view(key, elem, fragment) =
  if fragment
  then h.manifestElement(elem)
  else h.manifestPage(
    h.html({}, [
      h.head({}, [
        h.script({ src: 'https://unpkg.com/htmx.org@2.0.4', integrity: 'sha384-HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+', crossorigin: 'anonymous' }),
        h.script({ src: 'https://unpkg.com/htmx-ext-ws@2.0.2', integrity: 'sha384-932iIqjARv+Gy0+r6RTGrfCkCKS5MsF539Iqf6Vt8L4YmbnnWI2DSFoMD90bvXd0', crossorigin: 'anonymous' }),
      ]),
      h.body({}, [
        h.div({ 'hx-ext': 'ws', 'ws-connect': '/ws/%(key)s' % { key: key } }, [
          elem,
        ]),
      ]),
    ])
  );

view
