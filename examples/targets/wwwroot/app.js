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
  children.push(n.component(new ModelList(model, (key, value) => {
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
    return new Pair(n.elem('th', {}, [n.text(key)]), n.elem('td', {}, [td]))
  }, {tagName: 'tbody', subTagName: 'tr'})));
  return n.elem('div', [n.elem('table', {className: 'keyPairs'}, children)]);
}

// Get the collection from the service.
client.get('inventory.targets?target=my').then(targets => {
  // Here we use modapp components to render the view.
  // It is a protest against all these frameworks with virtual doms.
  // Why use virtual doms when it is faster with vanilla javascript?
  new Elem(n =>
    n.component(new CollectionList(targets, target => {
      const dds = [];
      dds.push(n.elem('dt', [n.text("Realm")]));
      dds.push(n.elem('dd', [n.component(new ModelTxt(target, target => target.realm))]));
      if (target.name) {
        dds.push(n.elem('dt', [n.text("Name")]));
        dds.push(n.elem('dd', [n.component(new ModelTxt(target, target => target.name))]));
      }
      dds.push(n.elem('dt', [n.text("URI")]));
      dds.push(n.elem('dd', [n.component(new ModelTxt(target, target => target.uri))]));

      if (target.config) {
        dds.push(n.elem('dt', [n.text("Config")]));
        dds.push(n.elem('dd', [makeRecursiveMapModel(n, target.config)]));
      }

      if (target.facts) {
        dds.push(n.elem('dt', [n.text("Facts")]));
        dds.push(n.elem('dd', [makeRecursiveMapModel(n, target.facts)]));
      }

      if (target.vars) {
        dds.push(n.elem('dt', [n.text("Vars")]));
        dds.push(n.elem('dd', [makeRecursiveMapModel(n, target.vars)]));
      }

      const c = new Elem(n =>
        n.elem('div', {className: 'list-item'}, [
          n.elem('div', {className: 'card shadow'}, [
            n.elem('dl', {className: 'view'}, dds)
          ])
        ])
      );
      return c;
    }, {className: 'list'}))
  ).render(document.getElementById('targets'));
}).catch(err => showError(err.code === 'system.connectionError'
  ? "Connection error. Are NATS Server and Resgate running?"
  : err
));
