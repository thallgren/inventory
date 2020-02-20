import ModelList from "./ModelList.js";
import Pair from "./Pair.js";

const {Elem, Txt, Button, Input} = window["modapp-base-component"];
const {CollectionList, ModelTxt, ModelComponent} = window["modapp-resource-component"];
const ResClient = resclient.default;
const ResCollection = resclient.ResCollection;
const ResModel = resclient.ResModel;

let client = new ResClient('ws://localhost:8080');

// Error handling
let errMsg = new Txt();
let errTimer = null;
errMsg.render(document.getElementById('error-msg'));
let showError = (err) => {
  errMsg.setText(err && err.message ? err.message : String(err));
  clearTimeout(errTimer);
  errTimer = setTimeout(() => errMsg.setText(''), 7000);
};

function makeRecursiveMapModel(n, model, caption) {
  const children = [];
  if (caption !== undefined) {
    children.push(n.elem('caption', [n.text(caption)]));
  }
  children.push(n.component(new ModelList(
    model,
    (key, value) => makePair(n, key, value, 'th', 'td'),
    {tagName: 'tbody', subTagName: 'tr'})));
  return n.elem('div', [n.elem('table', {className: 'keyPairs'}, children)]);
}

function makePair(n, key, value, keyTag, valueTag) {
  let td;
  if (value instanceof ResModel) {
    td = makeRecursiveMapModel(n, value)
  } else if (value instanceof ResCollection) {
    td = n.component(new CollectionList(values, value => {
      if (value instanceof ResModel) {
        return makeRecursiveMapModel(n, value);
      } else {
        return n.text(value)
      }
    }))
  } else {
    td = n.text(value)
  }
  return new Pair(n.elem(keyTag, {}, [n.text(key)]), n.elem(valueTag, {}, [td]))
}

function topElems(target) {
  return new Elem(n => {
    return n.elem('div', {className: 'list-item'}, [
      n.elem('div', {className: 'card shadow'}, [
        n.component(new ModelList(
          target,
          (key, value) => makePair(n, key, value, 'dt', 'dd'),
          {include: ['realm', 'name', 'uri', 'config', 'facts', 'vars', 'features'], className: 'view', tagName: 'dl', subTagName: 'div'}))])]);
  })
}

// Get the collection from the service.
client.get('inventory.targets').then(targets => {
  return new CollectionList(targets, target => {
    return new ModelComponent(target, topElems(target))
    }, {className: 'list'}
  ).render(document.getElementById('targets'));
}).catch(err => showError(err.code === 'system.connectionError'
  ? "Connection error. Are NATS Server and Resgate running?"
  : err
));
