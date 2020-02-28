"use strict";
const {Elem, Pair, Txt} = window["modapp-base-component"];
const {CollectionList, ModelList, ModelComponent} = window["modapp-resource-component"];
const {ResCollection, ResModel} = window["resclient"]
const ResClient = resclient.default;

function makeRecursiveMapModel(n, model) {
  return n.elem('table', {className: 'keyPairs'}, [
    n.component(
      new ModelList(model, (key, value) => makePair(key, value, 'th', 'td'),
        {tagName: 'tbody', subTagName: 'tr'}))
  ]);
}

function makePair(key, value, keyTag, valueTag) {
  return new Pair(
    new Elem(n => n.elem(keyTag, [n.text(key)])),
    new Elem(n => {
      let td;
      if (value instanceof ResModel) {
        td = makeRecursiveMapModel(n, value)
      } else if (value instanceof ResCollection) {
        td = n.component(new CollectionList(value,
          value => (value instanceof ResModel) ? makeRecursiveMapModel(n, value) : new Txt(value)))
      } else {
        td = n.text(value)
      }
      return n.elem(valueTag, [td])
    }));
}

function makeTopElems(targets) {
  return new CollectionList(targets, target => {
    return new ModelComponent(target, new Elem(n => n.elem('div', {className: 'card shadow'}, [
      n.component(new ModelList(
        target,
        (key, value) => makePair(key, value, 'dt', 'dd'), {
          include: ['realm', 'name', 'uri', 'config', 'facts', 'vars', 'features'],
          className: 'view',
          tagName: 'dl'
        }
      ))
    ])))
  }).render(document.getElementById('targets'));
}

// Error handling
const errMsg = new Txt();
let errTimer = null;
errMsg.render(document.getElementById('error-msg'));
let showError = (err) => {
  errMsg.setText(err && err.message ? err.message : String(err));
  clearTimeout(errTimer);
  errTimer = setTimeout(() => errMsg.setText(''), 7000);
};

// Get the collection from the service.
const client = new ResClient('ws://home.tada.se:8080');
client.get('inventory.targets').then(makeTopElems).catch(err => showError(err.code === 'system.connectionError'
  ? "Connection error. Are NATS Server and Resgate running?"
  : err
));
