{
  trigger: '',
  output: '',
  app: {
    subscriptions: [
      $.trigger,
    ],
    update: {
      [$.trigger](model, msg): {
        [$.output]:
          if msg.action == 'single' then {
            brightness: 255,
            state: 'on',
          } else if msg.action == 'double' then {
            brightness: 127,
            state: 'on',
          } else {
            brightness: 0,
            state: 'off',
          },
      },
    },
  },
}
