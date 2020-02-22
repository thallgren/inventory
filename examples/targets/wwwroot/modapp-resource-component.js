(function (global, factory) {
	typeof exports === 'object' && typeof module !== 'undefined' ? factory(exports, require('modapp-base-component')) :
	typeof define === 'function' && define.amd ? define(['exports', 'modapp-base-component'], factory) :
	(global = global || self, factory(global['modapp-resource-component'] = {}, global['modapp-base-component']));
}(this, (function (exports, modappBaseComponent) { 'use strict';

	/**
	 * A module with animation helper functions
	 */

	var FADE_DURATION = 200;

	var animate = function animate(progress, duration, step, done, token) {
		if (progress === 1) {
			done();
			return null;
		}

		var startProgress = progress;
		var start = null;
		token = token || {};
		var cb = function cb(timestamp) {
			if (!start) {
				start = timestamp - duration * startProgress;
			}

			progress = (timestamp - start) / duration;
			if (progress >= 1) {
				delete token.requestId;
				return done();
			}

			step(progress);
			token.requestId = window.requestAnimationFrame(cb);
		};

		token.requestId = window.requestAnimationFrame(cb);
		return token;
	};

	var slideDone = function slideDone(el, show, callback) {
		el.style.opacity = '';
		el.style.height = '';
		el.style.width = '';
		el.style.overflow = '';
		el.style.display = show ? '' : 'none';
		if (callback) {
			callback();
		}
		return null;
	};

	var invert = function invert(v, ok) {
		return ok ? 1 - v : v;
	};

	var easeOut = function easeOut(p) {
		return 1 - (1 - p) * (1 - p);
	};

	var unstyledCbs = null;
	var getUnstyledHeight = function getUnstyledHeight(el, cb) {
		if (unstyledCbs !== null) {
			unstyledCbs.push([el, cb, '', 0]);
			return;
		}

		unstyledCbs = [[el, cb, '', 0]];
		window.requestAnimationFrame(function () {
			var cs = unstyledCbs;
			unstyledCbs = null;
			// Reset all styles
			var _iteratorNormalCompletion = true;
			var _didIteratorError = false;
			var _iteratorError = undefined;

			try {
				for (var _iterator = cs[Symbol.iterator](), _step; !(_iteratorNormalCompletion = (_step = _iterator.next()).done); _iteratorNormalCompletion = true) {
					var c = _step.value;

					c[2] = c[0].style.cssText;
					c[0].style.cssText = '';
				}

				// Check calculated heights
			} catch (err) {
				_didIteratorError = true;
				_iteratorError = err;
			} finally {
				try {
					if (!_iteratorNormalCompletion && _iterator.return) {
						_iterator.return();
					}
				} finally {
					if (_didIteratorError) {
						throw _iteratorError;
					}
				}
			}

			var _iteratorNormalCompletion2 = true;
			var _didIteratorError2 = false;
			var _iteratorError2 = undefined;

			try {
				for (var _iterator2 = cs[Symbol.iterator](), _step2; !(_iteratorNormalCompletion2 = (_step2 = _iterator2.next()).done); _iteratorNormalCompletion2 = true) {
					var _c = _step2.value;

					_c[3] = _c[0].offsetHeight;
				}

				// Reset all styles
			} catch (err) {
				_didIteratorError2 = true;
				_iteratorError2 = err;
			} finally {
				try {
					if (!_iteratorNormalCompletion2 && _iterator2.return) {
						_iterator2.return();
					}
				} finally {
					if (_didIteratorError2) {
						throw _iteratorError2;
					}
				}
			}

			var _iteratorNormalCompletion3 = true;
			var _didIteratorError3 = false;
			var _iteratorError3 = undefined;

			try {
				for (var _iterator3 = cs[Symbol.iterator](), _step3; !(_iteratorNormalCompletion3 = (_step3 = _iterator3.next()).done); _iteratorNormalCompletion3 = true) {
					var _c2 = _step3.value;

					_c2[0].style.cssText = _c2[2];
				}

				// Call all callbacks
			} catch (err) {
				_didIteratorError3 = true;
				_iteratorError3 = err;
			} finally {
				try {
					if (!_iteratorNormalCompletion3 && _iterator3.return) {
						_iterator3.return();
					}
				} finally {
					if (_didIteratorError3) {
						throw _iteratorError3;
					}
				}
			}

			var _iteratorNormalCompletion4 = true;
			var _didIteratorError4 = false;
			var _iteratorError4 = undefined;

			try {
				for (var _iterator4 = cs[Symbol.iterator](), _step4; !(_iteratorNormalCompletion4 = (_step4 = _iterator4.next()).done); _iteratorNormalCompletion4 = true) {
					var _c3 = _step4.value;

					_c3[1](_c3[3]);
				}
			} catch (err) {
				_didIteratorError4 = true;
				_iteratorError4 = err;
			} finally {
				try {
					if (!_iteratorNormalCompletion4 && _iterator4.return) {
						_iterator4.return();
					}
				} finally {
					if (_didIteratorError4) {
						throw _iteratorError4;
					}
				}
			}
		});
	};

	/**
	 * Slides down an element while while fading it in.
	 * @param {HTMLElement} el HTML element to slide up/down.
	 * @param {boolean} show Flag if element should be slide up (show), will slide down (hide) if false.
	 * @param {object} [opt] Optional parameters
	 * @param {number} [opt.duration] Optional fade duration in milliseconds.
	 * @param {number} [opt.reset] Optional reset flag. If true, opacity and position will be reset. If false, animation will continue from current height and opacity. Default is true.
	 * @param {function} [opt.callback] Optional callback function once the animation is complete.
	 * @returns {object} Animation token
	 */
	var slideVertical = function slideVertical(el, show) {
		var opt = arguments.length > 2 && arguments[2] !== undefined ? arguments[2] : {};

		var token = { requestId: true };
		var progress = 0;
		var origin = void 0,
		    target = void 0,
		    height = void 0,
		    e = void 0;
		var reset = opt.reset !== undefined ? opt.reset : true;
		var f = reset || show ? getUnstyledHeight : function (el, cb) {
			return cb(0);
		};

		f(el, function (unstyledHeight) {
			if (!token.requestId) {
				return;
			}

			if (reset) {
				el.style.opacity = show ? 0 : 1;
				target = show ? unstyledHeight : 0;
				origin = show ? 0 : unstyledHeight;
				el.style.height = origin + 'px';
			} else {
				progress = invert(el.style.opacity ? parseFloat(el.style.opacity) : el.style.display === 'none' ? 0 : 1, !show);

				if (progress === 1) {
					return slideDone(el, show, opt.callback);
				}

				target = show ? unstyledHeight : 0;

				e = easeOut(progress);
				height = el.style.display === 'none' ? 0 : el.offsetHeight;
				origin = (height - e * target) / (1 - e);
			}

			el.style.display = '';
			el.style.overflow = 'hidden';

			animate(progress, opt.duration || FADE_DURATION, function (p) {
				e = easeOut(p);
				el.style.opacity = show ? p : 1 - p;
				el.style.height = e * target + (1 - e) * origin + 'px';
			}, function () {
				return slideDone(el, show, opt.callback);
			}, token);
		});

		return token;
	};

	/**
	 * Stops the animation for the given token.
	 * @param {object} token Animation token.
	 * @returns {boolean} True if the animation was active, otherwise false.
	 */
	var stop = function stop(token) {
		if (token && token.requestId) {
			if (token.requestId !== true) {
				window.cancelAnimationFrame(token.requestId);
			}
			delete token.requestId;
			return true;
		}
		return false;
	};

	/**
	 * A component rendering a list of items based on a collection
	 */

	class CollectionList extends modappBaseComponent.RootElem {
	  /**
	   * Creates an instance of CollectionList
	   * @param {Collection} collection Iterable list of items
	   * @param {function} componentFactory  A factory function taking a collection item as argument, returning a component.
	   * @param {object} [opt] Optional parameters.
	   * @param {string} [opt.tagName] Tag name (eg. 'ul') for the element. Defaults to 'div'.
	   * @param {string} [opt.className] Class name
	   * @param {object} [opt.attributes] Key/value attributes object
	   * @param {object} [opt.events] Key/value events object, where the key is the event name, and value is the callback.
	   * @param {string} [opt.subTagName] Tag name (eg. 'li') for the element. Defaults to 'div'.
	   * @param {string} [opt.subClassName] A factory function taking a collection item as argument, returning the className for the component.
	   */
	  constructor(collection, componentFactory, opt) {
	    opt = Object.assign({
	      tagName: 'div'
	    }, opt);
	    super(opt.tagName, opt);
	    this.collection = null;
	    this.componentFactory = componentFactory;
	    this.subTagName = opt.subTagName || 'div';
	    this.subClassName = opt.subClassName || null;
	    this.components = null;
	    this.removedComponents = [];
	    this._add = this._add.bind(this);
	    this._remove = this._remove.bind(this);
	    this._rel = null; // Root elements node

	    this.setCollection(collection);
	  }
	  /**
	   * Sets the collection.
	   * If the component is rendered, the list will be rerendered with
	   * the new collection, without any animation.
	   * @param {?Collection} collection Iterable list of items
	   * @returns {this}
	   */


	  setCollection(collection) {
	    collection = collection || null;

	    if (collection === this.collection) {
	      return this;
	    }

	    if (!this._rel) {
	      this.collection = collection;
	      return this;
	    }

	    this._unrenderComponents();

	    this.collection = collection;

	    this._renderComponents();

	    this._checkSync();

	    return this;
	  }
	  /**
	   * Gets the current collection
	   * @returns {?Collection}
	   */


	  getCollection() {
	    return this.collection;
	  }
	  /**
	   * Get the component for a model by index
	   * @param {number} idx Index if model
	   * @returns {?Component} Model component, or null if the list isn't rendered, or if index is out of bounds
	   */


	  getComponent(idx) {
	    if (!this._rel) {
	      return null;
	    }

	    let cont = this.components[idx];
	    return cont ? cont.component : null;
	  }
	  /**
	   * Waits for the synchronization of the collection and component list to
	   * ensure the collection models matches the rendered components.
	   * Calling this method is necessary when calling getComponent after
	   * adding/removing items from the collections.
	   * Callback will never be called if the CollectionList isn't rendered, or
	   * if it unrenders before it has been synchronized.
	   * @param {function} callback Callback function called when collection and component list is synchronized.
	   */


	  sync(callback) {
	    if (!this._rel) {
	      return;
	    }

	    if (this._syncCallbacks) {
	      this._syncCallbacks.push(callback);
	    } else {
	      this._syncCallbacks = [callback];
	    }

	    this._checkSync();
	  }

	  render(el) {
	    this._rel = super.render(el);

	    this._renderComponents();

	    return this._rel;
	  }

	  unrender() {
	    this._unrenderComponents();

	    this._syncCallbacks = null;
	    super.unrender();
	    this._rel = null;
	  }

	  _checkSync() {
	    // No use checking syncronization if noone cares.
	    if (!this._syncCallbacks) {
	      return;
	    }

	    let i = 0,
	        comp,
	        len = this.components.length;

	    for (let model of this.collection) {
	      // More models in the collection than components
	      if (i === len) {
	        return;
	      }

	      comp = this.components[i++];

	      if (model !== comp.model) {
	        return;
	      }
	    } // Do we have more components?


	    if (i !== length) {
	      return;
	    } // We are in sync


	    for (let cb of this._syncCallbacks) {
	      cb();
	    }

	    this._syncCallbacks = null;
	  }

	  _setSubClassName(item, li) {
	    if (this.subClassName) {
	      let className = this.subClassName(item);

	      if (className) {
	        li.className = className;
	      }
	    }
	  }

	  _renderComponents() {
	    if (!this.collection) {
	      return;
	    }

	    this.components = [];
	    let idx = 0;

	    for (let item of this.collection) {
	      let component = this.componentFactory(item, idx);
	      let li = document.createElement(this.subTagName);
	      this.components.push({
	        item,
	        component,
	        li
	      });

	      this._setSubClassName(item, li);

	      this._rel.appendChild(li);

	      if (component) {
	        component.render(li);
	      }

	      idx++;
	    }

	    this._setEventListener(true);
	  }

	  _unrenderComponents() {
	    if (!this.collection) {
	      return;
	    }

	    for (let cont of this.components) {
	      this._removeComponent(cont);
	    }

	    this.components = null;

	    for (let cont of this.removedComponents) {
	      this._removeComponent(cont);
	    }

	    this.removedComponents = [];

	    this._setEventListener(false);
	  } // Callback when the collection have an add event


	  _add(e) {
	    // Assert component wasn't unrendered by another event handler
	    if (!this._rel) {
	      return;
	    }

	    let {
	      item,
	      idx
	    } = e;
	    let component = this.componentFactory(item, idx);
	    let li = document.createElement(this.subTagName);
	    let cont = {
	      model: item,
	      component,
	      li
	    };
	    this.components.splice(idx, 0, cont);

	    this._setSubClassName(item, li);

	    li.style.display = 'none'; // Append last?

	    if (this.components.length - 1 === idx) {
	      this._rel.appendChild(li);
	    } else {
	      this._rel.insertBefore(li, this.components[idx + 1].li);
	    }

	    if (component) {
	      component.render(li);
	    }

	    cont.token = slideVertical(li, true, {
	      reset: true
	    });

	    this._checkSync();
	  } // Callback when the collection have a remove event


	  _remove(e) {
	    // Assert component wasn't unrendered by another event handler
	    if (!this._rel) {
	      return;
	    }

	    let cont = this.components[e.idx];
	    this.components.splice(e.idx, 1);
	    this.removedComponents.push(cont);
	    stop(cont.token);
	    cont.token = slideVertical(cont.li, false, {
	      callback: () => {
	        let idx = this.removedComponents.indexOf(cont);

	        if (idx >= 0) {
	          this.removedComponents.splice(idx, 1);

	          this._removeComponent(cont);
	        }
	      }
	    });

	    this._checkSync();
	  }

	  _removeComponent(cont) {
	    if (!this._rel) {
	      return;
	    }

	    let {
	      token,
	      component
	    } = cont;
	    stop(token);

	    if (component) {
	      component.unrender();
	    }

	    this._rel.removeChild(cont.li);
	  }

	  _setEventListener(on) {
	    if (this.collection && this.collection.on) {
	      if (on) {
	        this.collection.on('add', this._add);
	        this.collection.on('remove', this._remove);
	      } else {
	        this.collection.off('add', this._add);
	        this.collection.off('remove', this._remove);
	      }
	    }
	  }

	}

	/**
	 * Select option object
	 * @typedef {Object} CollectionSelect~option
	 * @property {string} text Option text.
	 * @property {string} [value] Option value.
	 */

	/**
	* Option map callback
	* @callback CollectionSelect~optionMap
	* @param {*} item Item from the collection.
	* @returns {CollectionSelect~option} responseMessage
	*/

	/**
	 * A select component with options based on a Collection.
	 */

	class CollectionSelect extends modappBaseComponent.RootElem {
	  /**
	   * Creates an instance of Select
	   * @param {Iterator} collection Collection of options.
	   * @param {CollectionSelect~optionMap} [optionMap] Map function that takes a collection item and returns an option object.
	   * @param {object} [opt] Optional parameters.
	   * @param {string} [opt.selected] Default selected value.
	   * @param {string} [opt.className] Class name.
	   * @param {object} [opt.attributes] Key/value attributes object.
	   * @param {object} [opt.events] Key/value events object, where the key is the event name, and value is the callback.
	   * @param {function} [opt.optionFactory] Function that builds an option Component from a collection item, or option object if optionMap is defined.
	   * @param {CollectionSelect~option} [opt.placeholder] Placeholder option object. Eg. { text: "Choose one", value: "" }
	   */
	  constructor(collection, optionMap, opt) {
	    if (typeof optionMap != 'function') {
	      opt = optionMap;
	      optionMap = null;
	    }

	    opt = Object.assign({
	      optionFactory: o => new modappBaseComponent.Txt(o.text, {
	        tagName: 'option',
	        attributes: {
	          value: o.value
	        }
	      })
	    }, opt);
	    super('select', opt);

	    this.optionMap = optionMap || (o => o);

	    this.collection = collection || null;
	    this.optionFactory = opt.optionFactory;
	    this.placeholder = opt.placeholder;
	    this.setSelected(opt.selected); // Bind callbacks

	    this._update = this._update.bind(this);
	  }
	  /**
	   * Sets the selected option.
	   * @param {string} value The value of the option to set as selected.
	   * @returns {this}
	   */


	  setSelected(value) {
	    if (typeof value != 'string') {
	      value = value ? String(value) : null;
	    }

	    super.setProperty('value', value);
	    this.selected = value;
	    return this;
	  }
	  /**
	   * Gets the selected option.
	   * @returns {string} The value of the selected option.
	   */


	  getSelected() {
	    return super.getElement() ? super.getProperty('value') : this.selected;
	  }
	  /**
	   * Sets the collection of options.
	   * @param {Iterator<T>} collection Collection of options.
	   */


	  setCollection(collection) {
	    if (!super.getElement()) {
	      this.collection = collection || null;
	      return;
	    }

	    this._setEventListener(false);

	    this.collection = collection || null;

	    this._update();

	    this._setEventListener(true);

	    return this;
	  }

	  render(el) {
	    super.render(el);
	    let rel = super.getElement();

	    this._renderOptions(rel);

	    this._setEventListener(true);

	    super.setProperty('value', this.selected);
	    return rel;
	  }

	  unrender() {
	    this.selected = super.getProperty('value');

	    this._setEventListener(false);

	    this._unrenderOptions();

	    super.unrender();
	  }

	  _renderOptions(rel) {
	    if (this.placeholder) {
	      let c = this.optionFactory(this.placeholder);
	      this.placeholderComponent = c;
	      c.render(rel);
	    }

	    this.optionComponents = [];

	    if (this.collection) {
	      for (let o of this.collection) {
	        let c = this.optionFactory(this.optionMap(o));
	        let el = c.render(rel);
	        this.optionComponents.push({
	          c,
	          el
	        });
	      }
	    }
	  }

	  _unrenderOptions() {
	    if (this.placeholderComponent) {
	      this.placeholderComponent.unrender();
	      this.placeholderComponent = null;
	    }

	    for (let oc of this.optionComponents) {
	      oc.c.unrender();
	    }

	    this.optionComponents = null;
	  }

	  _setEventListener(on) {
	    if (this.collection && this.collection.on) {
	      if (on) {
	        this.collection.on('add', this._update);
	        this.collection.on('remove', this._update);
	      } else {
	        this.collection.off('add', this._update);
	        this.collection.off('remove', this._update);
	      }
	    }
	  }

	  _update() {
	    let rel = super.getElement();

	    if (rel) {
	      this.selected = super.getProperty('value');

	      this._unrenderOptions();

	      this._renderOptions(rel);

	      super.setProperty('value', this.selected);
	    }
	  }

	}

	/**
	 * Update callback function
	 * @callback ModelListener~updateCallback
	 * @param {Model} model Model
	 * @param {Component} component Component received in ModelListener constructor.
	 * @param {?object} changed Model's change event parameters. Null if the callback is triggered by render or setModel.
	 */

	/**
	 * A helper class for model components
	 */
	class ModelListener {
	  /**
	   * Creates a new ModelListener instance
	   * @param {Model} [model] Optional model object
	   * @param {Component} component Component
	   * @param {ModelListener~updateCallback} update Callback function called on model change and when component is rendered
	   */
	  constructor(model, component, update) {
	    this.model = model;
	    this.component = component;
	    this.update = update;
	    this.rendered = false;
	    this._onModelChange = this._onModelChange.bind(this);
	  }

	  onRender() {
	    this._setEventListener(true);

	    this._onModelChange(null);

	    this.rendered = true;
	  }

	  onUnrender() {
	    this._setEventListener(false);

	    this.rendered = false;
	  }
	  /**
	   * Set model
	   * If component is rendered, update will be triggered.
	   * @param {?Model} model Model
	   * @returns {this}
	   */


	  setModel(model) {
	    model = model || null;

	    if (model === this.model) {
	      return;
	    }

	    if (this.rendered) {
	      this._setEventListener(false);

	      this.model = model;

	      this._setEventListener(true);

	      this._onModelChange(null);
	    } else {
	      this.model = model;
	    }

	    return this;
	  }

	  _setEventListener(on) {
	    if (!this.model || !this.model.on || !this.update) {
	      return;
	    }

	    if (on) {
	      this.model.on('change', this._onModelChange);
	    } else {
	      this.model.off('change', this._onModelChange);
	    }
	  }

	  _onModelChange(changed) {
	    if (this.update) {
	      this.update(this.model, this.component, changed);
	    }
	  }

	}

	/**
	 * A button component based on an model
	 */

	class ModelButton extends modappBaseComponent.Button {
	  /**
	   * Creates an instance of ModelButton
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a string is returned, it will set the text of the button.
	   * @param {function} click Click callback. Will pass itself and the event as argument on callback.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Button.
	   */
	  constructor(model, update, click, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, click, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    this.setText(this.update(m, c, changed));
	  }

	}

	/**
	 * A checkbox component based on an model
	 */

	class ModelCheckbox extends modappBaseComponent.Checkbox {
	  /**
	   * Creates an instance of ModelCheckbox
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a boolean is returned, it will be used to check/uncheck the checkbox.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Checkbox.
	   */
	  constructor(model, update, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    let result = this.update(m, c, changed);

	    if (typeof result === 'boolean') {
	      this.setChecked(result);
	    }
	  }

	}

	/**
	 * Update callback function
	 * @callback ModelComponent~updateCallback
	 * @param {Model} model Model
	 * @param {Component} component Component
	 * @param {?object} changed Model's change event parameters. Null if the callback is triggered by render or setModel.
	 */

	/**
	 * A generic component wrapper that listens to change events on a model, calling update on change.
	 */

	class ModelComponent {
	  /**
	   * Creates a new ModelComponent instance
	   * @param {Model} [model] Optional model object
	   * @param {Component} component Component
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered
	   */
	  constructor(model, component, update) {
	    if (typeof component === 'function') {
	      update = component;
	      component = model;
	      model = null;
	    }

	    this.ml = new ModelListener(model, component, update);
	  }
	  /**
	   * Set model
	   * If component is rendered, update will be triggered.
	   * @param {?Model} model Model
	   * @returns {this}
	   */


	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  render(el) {
	    this.ml.onRender();
	    return this.ml.component.render(el);
	  }

	  unrender() {
	    this.ml.component.unrender();
	    this.ml.onUnrender();
	  }

	}

	/**
	 * A input component based on an model
	 */

	class ModelInput extends modappBaseComponent.Input {
	  /**
	   * Creates an instance of ModelInput
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a string is returned, it will be used as the value of the input.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Input.
	   */
	  constructor(model, update, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    let result = this.update(m, c, changed);

	    if (typeof result === 'string') {
	      this.setValue(result);
	    }
	  }

	}

	/**
	 * A component rendering a list of key/value pairs based on a model
	 */

	class ModelList extends modappBaseComponent.RootElem {
	  /**
	      * Creates an instance of ModelList
	      * @param {object} model object
	      * @param {function} componentFactory  A factory function taking (key, value) as argument, returning a component.
	      * @param {object} [opt] Optional parameters.
	      * @param {string} [opt.tagName] Tag name (eg. 'ul') for the element. Defaults to 'div'.
	      * @param {string} [opt.className] Class name
	      * @param {object} [opt.attributes] Key/value attributes object
	      * @param {object} [opt.events] Key/value events object, where the key is the event name, and value is the callback.
	      * @param {string[]} [opt.exclude] Arrays of keys to exclude
	      * @param {string[]} [opt.include] Arrays of keys to include. If present, also determines order
	      * @param {string} [opt.subTagName] Tag name (eg. 'li') for the element. Defaults to 'div'.
	      * @param {string} [opt.subClassName] A factory function taking a collection item as argument, returning the className for the component.
	      */
	  constructor(model, componentFactory, opt) {
	    opt = Object.assign({
	      tagName: 'div'
	    }, opt);
	    super(opt.tagName, opt);
	    this.collection = null;
	    this.componentFactory = componentFactory;
	    this.subTagName = opt.subTagName || 'div';
	    this.subClassName = opt.subClassName || null;
	    this.exclude = opt.exclude || null;
	    this.include = opt.include || null;
	    this.components = null;
	    this.removedComponents = [];
	    this._change = this._change.bind(this);
	    this._rel = null; // Root elements node

	    this.setModel(model);
	  }
	  /**
	      * Sets the model.
	      * If the component is rendered, the list will be rerendered with
	      * the new collection, without any animation.
	      * @param {?object} model map of items
	      * @returns {this}
	      */


	  setModel(model) {
	    model = model || null;

	    if (model === this.model) {
	      return this;
	    }

	    if (!this._rel) {
	      this.model = model;
	      return this;
	    }

	    this._unrenderComponents();

	    this.model = model;

	    this._renderComponents();

	    this._checkSync();

	    return this;
	  }
	  /**
	      * Gets the current model
	      * @returns {?object}
	      */


	  getModel() {
	    return this.model;
	  }
	  /**
	      * Get the component for a model by index
	      * @param {number} idx Index if model
	      * @returns {?Component} Model component, or null if the list isn't rendered, or if index is out of bounds
	      */


	  getComponent(idx) {
	    if (!this._rel) {
	      return null;
	    }

	    let cont = this.components[idx];
	    return cont ? cont.component : null;
	  }
	  /**
	      * Waits for the synchronization of the collection and component list to
	      * ensure the collection models matches the rendered components.
	      * Calling this method is necessary when calling getComponent after
	      * adding/removing items from the collections.
	      * Callback will never be called if the CollectionList isn't rendered, or
	      * if it unrenders before it has been synchronized.
	      * @param {function} callback Callback function called when collection and component list is synchronized.
	      */


	  sync(callback) {
	    if (!this._rel) {
	      return;
	    }

	    if (this._syncCallbacks) {
	      this._syncCallbacks.push(callback);
	    } else {
	      this._syncCallbacks = [callback];
	    }

	    this._checkSync();
	  }

	  render(el) {
	    this._rel = super.render(el);

	    this._renderComponents();

	    return this._rel;
	  }

	  unrender() {
	    this._unrenderComponents();

	    this._syncCallbacks = null;
	    super.unrender();
	    this._rel = null;
	  } // produces an array of strings corresponding to the keys of the contained model. The array
	  // is sorted and adjusted according to the include and exclude options.
	  // @returns {string[]}


	  _orderedKeys() {
	    const props = this.model.props;
	    let keys;
	    const ic = this.include;

	    if (ic === null) {
	      keys = Reflect.ownKeys(props);
	      keys.sort();
	    } else {
	      keys = [];

	      for (let i = 0; i < ic.length; i++) {
	        const key = ic[i];

	        if (props.hasOwnProperty(key)) {
	          keys.push(key);
	        }
	      }
	    }

	    const ex = this.exclude;

	    if (ex !== null) {
	      keys = keys.filter(function (key) {
	        return ex.includes(key);
	      });
	    }

	    return keys;
	  }

	  _checkSync() {
	    // No use checking syncronization if noone cares.
	    if (!this._syncCallbacks) {
	      return;
	    }

	    const keys = this._orderedKeys();

	    if (keys.length !== this.components.length) {
	      return;
	    }

	    const props = this.model.props;

	    for (let i = 0; i < keys.length; i++) {
	      const comp = this.components[i];

	      if (props[keys[i]] !== comp.model) {
	        return;
	      }
	    } // We are in sync


	    for (let cb of this._syncCallbacks) {
	      cb();
	    }

	    this._syncCallbacks = null;
	  }

	  _setSubClassName(item, li) {
	    if (this.subClassName) {
	      const className = this.subClassName(item);

	      if (className) {
	        li.className = className;
	      }
	    }
	  }

	  _renderComponents() {
	    if (!this.model) {
	      return;
	    }

	    this.components = [];
	    const props = this.model.props;

	    const keys = this._orderedKeys();

	    for (let idx = 0; idx < keys.length; idx++) {
	      const key = keys[idx];
	      const item = props[key];
	      let component = this.componentFactory(key, item);
	      let li = document.createElement(this.subTagName);
	      this.components.push({
	        item,
	        component,
	        li,
	        idx,
	        key
	      });

	      this._setSubClassName(item, li);

	      this._rel.appendChild(li);

	      if (component) {
	        component.render(li);
	      }
	    }

	    this._setEventListener(true);
	  }

	  _unrenderComponents() {
	    if (!this.model) {
	      return;
	    }

	    for (let cont of this.components) {
	      this._removeComponent(cont);
	    }

	    this.components = null;

	    for (let cont of this.removedComponents) {
	      this._removeComponent(cont);
	    }

	    this.removedComponents = [];

	    this._setEventListener(false);
	  } // Callback when the model have a change event


	  _change(e) {
	    // Assert component wasn't unrendered by another event handler
	    if (!this._rel) {
	      return;
	    }

	    const props = this.model.props; // for each key listed in the event, check what actually changed.

	    for (let key in Reflect.ownKeys(e)) {
	      // find component that corresponds to the key, if any.
	      let cont = null;

	      for (let ct of this.components) {
	        if (ct.key === key) {
	          cont = ct;
	          break;
	        }
	      }

	      const item = props[key];
	      let idx = -1;

	      if (cont !== null) {
	        if (item === undefined) {
	          // item was removed
	          this._remove(cont.idx);

	          continue;
	        }

	        idx = cont.idx;
	      } else {
	        // find index of new property in model
	        const keys = this._orderedKeys();

	        for (let pi = 0; pi < keys.length; pi++) {
	          if (keys[pi] === key) {
	            idx = pi;
	            break;
	          }
	        }

	        if (idx < 0) {
	          // not found? This should normally not happen since a remove would
	          // have resulted in the removal of an existing component. In any case,
	          // there's nothing to add.
	          continue;
	        }
	      }

	      let li;
	      let component = this.componentFactory(key, item);

	      if (cont === null) {
	        // add new component
	        li = document.createElement(this.subTagName);
	        cont = {
	          model: item,
	          component,
	          li,
	          idx,
	          key
	        };
	        this.components.splice(idx, 0, cont);

	        this._setSubClassName(item, li);

	        li.style.display = 'none'; // Append last?

	        if (this.components.length - 1 === idx) {
	          this._rel.appendChild(li);
	        } else {
	          this._rel.insertBefore(li, this.components[idx + 1].li);
	        }

	        component.render(li);
	        cont.token = slideVertical(li, true, {
	          reset: true
	        });
	      } else {
	        // replace component
	        cont.component.unrender();
	        cont.component = component;
	        this.components[idx] = cont;
	        component.render(cont.li);
	      }
	    }

	    this._checkSync();
	  } // called when the model entries are removed


	  _remove(idx) {
	    const cont = this.components[idx];
	    this.components.splice(idx, 1);
	    this.removedComponents.push(cont);
	    stop(cont.token);
	    cont.token = slideVertical(cont.li, false, {
	      callback: () => {
	        let idx = this.removedComponents.indexOf(cont);

	        if (idx >= 0) {
	          this.removedComponents.splice(idx, 1);

	          this._removeComponent(cont);
	        }
	      }
	    });

	    this._checkSync();
	  }

	  _removeComponent(cont) {
	    if (!this._rel) {
	      return;
	    }

	    let {
	      token,
	      component
	    } = cont;
	    stop(token);

	    if (component) {
	      component.unrender();
	    }

	    this._rel.removeChild(cont.li);
	  }

	  _setEventListener(on) {
	    if (this.model && this.model.on) {
	      if (on) {
	        this.model.on('change', this._change);
	      } else {
	        this.model.off('change', this._change);
	      }
	    }
	  }

	}

	/**
	 * A radio component based on an model
	 */

	class ModelRadio extends modappBaseComponent.Radio {
	  /**
	   * Creates an instance of ModelRadio
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a boolean is returned, it will be used to check/uncheck the radiobutton.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Radiobutton.
	   */
	  constructor(model, update, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    let result = this.update(m, c, changed);

	    if (typeof result === 'boolean') {
	      this.setChecked(result);
	    }
	  }

	}

	/**
	 * A textarea component based on an model
	 */

	class ModelTextarea extends modappBaseComponent.Textarea {
	  /**
	   * Creates an instance of ModelTextarea
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a string is returned, it will be used as the value of the textarea.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Textarea.
	   */
	  constructor(model, update, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    let result = this.update(m, c, changed);

	    if (typeof result === 'string') {
	      this.setValue(result);
	    }
	  }

	}

	/**
	 * A text component based on an model
	 */

	class ModelTxt extends modappBaseComponent.Txt {
	  /**
	   * Creates an instance of ModelTxt
	   * @param {object} [model] Optional model object
	   * @param {ModelComponent~updateCallback} update Callback function called on model change and when component is rendered. If a string is returned, it will set the text of the element.
	   * @param {object} [opt] Optional parameters for the underlying modapp-base-component/Txt.
	   */
	  constructor(model, update, opt) {
	    if (typeof model === 'function') {
	      opt = update;
	      update = model;
	      model = null;
	    }

	    super(null, opt);
	    this.update = update;
	    this.ml = new ModelListener(model, this, this._changeHandler.bind(this));
	  }

	  render(el) {
	    this.ml.onRender();
	    return super.render(el);
	  }

	  unrender() {
	    super.unrender();
	    this.ml.onUnrender();
	  }

	  setModel(model) {
	    this.ml.setModel(model);
	    return this;
	  }

	  _changeHandler(m, c, changed) {
	    this.setText(this.update(m, c, changed));
	  }

	}

	class Pair extends modappBaseComponent.RootElem {
	  /**
	      * Creates a new Pair instance
	      * @param {Elem~node} key Key node
	      * @param {Elem~node} value Value node
	      */
	  constructor(key, value) {
	    super();
	    this.key = new modappBaseComponent.Elem(key);
	    this.value = new modappBaseComponent.Elem(value);
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

	Object.defineProperty(exports, 'generateName', {
		enumerable: true,
		get: function () {
			return modappBaseComponent.generateName;
		}
	});
	exports.CollectionList = CollectionList;
	exports.CollectionSelect = CollectionSelect;
	exports.ModelButton = ModelButton;
	exports.ModelCheckbox = ModelCheckbox;
	exports.ModelComponent = ModelComponent;
	exports.ModelInput = ModelInput;
	exports.ModelList = ModelList;
	exports.ModelRadio = ModelRadio;
	exports.ModelTextarea = ModelTextarea;
	exports.ModelTxt = ModelTxt;
	exports.Pair = Pair;

	Object.defineProperty(exports, '__esModule', { value: true });

})));
