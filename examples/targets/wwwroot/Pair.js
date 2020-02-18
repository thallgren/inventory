const { RootElem, Elem } = window["modapp-base-component"];

class Pair extends RootElem {

   constructor(key, value) {
     super();
     this.key = new Elem(() => key);
     this.value = new Elem(() => value);
  }

  render(el) {
     this.key.render(el);
     this.value.render(el);
  }

  unrender() {
    this.key.unrender();
    this.value.unrender();
  }
}

export default Pair;
