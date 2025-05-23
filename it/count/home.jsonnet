local count = {
  trigger: 'yokai/test/input-a',
  output: 'yokai/test/output',
  app: {
    init: {
      value: 0,
    },
    subscriptions: [
      $.trigger,
    ],
    update: {
      [$.trigger](model, msg): {
        local update = self,
        model: {
          value: model.value + msg.add,
        },
        [$.output]: update.model,
      },
    },
  },
};

[count]
