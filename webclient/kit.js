(function() {
    var Global = null;
    if (typeof window !== "undefined") {
       Global = window;
    }

    var Protocol;
    if (Global && Global.Protocol) {
        Protocol = Global.Protocol;
    } else {
        Protocol = require('protocol.js');
    }

    var WebSocket;
    if (Global && Global.WebSocket) {
        WebSocket = Global.WebSocket;
    } else {
        WebSocket = require('wxwebsocket.js');
    }

    function Emitter(obj) {
        if (obj) return mixin(obj);
    }
    /**
     * Mixin the emitter properties.
     *
     * @param {Object} obj
     * @return {Object}
     * @api private
     */

    function mixin(obj) {
        for (var key in Emitter.prototype) {
            obj[key] = Emitter.prototype[key];
        }
        return obj;
    }

    /**
     * Listen on the given `event` with `fn`.
     *
     * @param {String} event
     * @param {Function} fn
     * @return {Emitter}
     * @api public
     */

    Emitter.prototype.on =
        Emitter.prototype.addEventListener = function(event, fn){
            this._callbacks = this._callbacks || {};
            (this._callbacks[event] = this._callbacks[event] || [])
                .push(fn);
            return this;
        };

    /**
     * Adds an `event` listener that will be invoked a single
     * time then automatically removed.
     *
     * @param {String} event
     * @param {Function} fn
     * @return {Emitter}
     * @api public
     */

    Emitter.prototype.once = function(event, fn){
        var self = this;
        this._callbacks = this._callbacks || {};

        function on() {
            self.off(event, on);
            fn.apply(this, arguments);
        }

        on.fn = fn;
        this.on(event, on);
        return this;
    };

    /**
     * Remove the given callback for `event` or all
     * registered callbacks.
     *
     * @param {String} event
     * @param {Function} fn
     * @return {Emitter}
     * @api public
     */

    Emitter.prototype.off =
        Emitter.prototype.removeListener =
        Emitter.prototype.removeAllListeners =
        Emitter.prototype.removeEventListener = function(event, fn){
            this._callbacks = this._callbacks || {};

            // all
            if (0 == arguments.length) {
                this._callbacks = {};
                return this;
            }

            // specific event
            var callbacks = this._callbacks[event];
            if (!callbacks) return this;

            // remove all handlers
            if (1 == arguments.length) {
                delete this._callbacks[event];
                return this;
            }

            // remove specific handler
            var cb;
            for (var i = 0; i < callbacks.length; i++) {
                cb = callbacks[i];
                if (cb === fn || cb.fn === fn) {
                    callbacks.splice(i, 1);
                    break;
                }
            }
            return this;
        };

    /**
     * Emit `event` with the given args.
     *
     * @param {String} event
     * @param {Mixed} ...
     * @return {Emitter}
     */

    Emitter.prototype.emit = function(event){
        this._callbacks = this._callbacks || {};
        var args = [].slice.call(arguments, 1)
            , callbacks = this._callbacks[event];

        if (callbacks) {
            callbacks = callbacks.slice(0);
            for (var i = 0, len = callbacks.length; i < len; ++i) {
                callbacks[i].apply(this, args);
            }
        }

        return this;
    };

    /**
     * Return array of callbacks for `event`.
     *
     * @param {String} event
     * @return {Array}
     * @api public
     */

    Emitter.prototype.listeners = function(event){
        this._callbacks = this._callbacks || {};
        return this._callbacks[event] || [];
    };

    /**
     * Check if this emitter has `event` handlers.
     *
     * @param {String} event
     * @return {Boolean}
     * @api public
     */

    Emitter.prototype.hasListeners = function(event){
        return !! this.listeners(event).length;
    };

    var Package = Protocol.Package;
    var Message = Protocol.Message;

    var reqId = 0;

    function KitSession(params, cb) {
        var self = this;

        if (typeof params === 'string') {
            params = {url:params};
        }

        if (!params.url) {
            throw new Error('params.url is needed');
        }

        self.sid = ''; //session id
        self.url = params.url;
        self.log = params.log;
        self._readyCb = cb;

        self._heartbeatInterval = 0;
        self._heartbeatTimer = null;

        self._requestCallbacks = {};
        self._delayBuffer = [];
        self._reconnectMaxAttempts = params.reconnectMaxAttempts || 10;
        self._reconnectDelay = params.reconnectDelay || 2;
        self._reconnectAttempts = 0;
        self._reconnectTimer = null;

        self._connect();
    }

    KitSession.prototype.__proto__ = Emitter.prototype;

    KitSession.Connecting = 'connecting';
    KitSession.Open = 'open';
    KitSession.Closed = 'closed';
    KitSession.Closing  = 'closing';

    KitSession.prototype.notify = function(route, msg) {
        var self = this;
        msg = msg || {};
        self._sendMsg(0, route, msg);
    };

    KitSession.prototype.request = function(route, msg, cb) {
        var self = this;
        if(arguments.length === 2 && typeof msg === 'function') {
            cb = msg;
            msg = {};
        } else {
            msg = msg || {};
        }
        route = route || msg.route;
        if(!route) {
            return;
        }

        reqId++;
        self._sendMsg(reqId, route, msg);

        if (reqId) {
            self._requestCallbacks[reqId] = cb;
        }
    };

    KitSession.prototype.disconnect = function() {
        var self = this;
        if (self.state == KitSession.Closing
                || self.state == KitSession.Closed) {
            return;
        }
        self.state = KitSession.Closing;

        self.log && console.log('KitSession.disconnect');

        self.emit('close');

        if (self.socket) {
            var pkt = Package.encode(Package.TYPE_KICK);
            self._send(pkt);
            setTimeout(function(){self._close();}, 100);
        }
    };

    KitSession.prototype._sendMsg = function(reqId, route, msg) {
        var self = this;
        var type = reqId ? Message.TYPE_REQUEST : Message.TYPE_NOTIFY;
        msg = Protocol.strencode(JSON.stringify(msg));
        msg = Message.encode(reqId, type, 0, route, msg);
        var packet = Package.encode(Package.TYPE_DATA, msg);
        if (self.state === KitSession.Open) {
            self._send(packet);
        } else {
            self._delayBuffer.push(packet);
        }
    };

    KitSession.prototype._close = function() {
        var self = this;

        self.state = KitSession.Closed

        if (self._heartbeatTimer) {
            clearInterval(self._heartbeatTimer);
            self._heartbeatTimer = null;
        }

        if (self._reconnectTimer) {
            clearTimeout(self._reconnectTimer);
            self._reconnectTimer = null;
        }

        if (self.socket) {
            self.socket.onopen = null;
            self.socket.onerror = null;
            self.socket.onmessage = null;
            self.socket.onclose = null;
            self.socket.close();
            self.socket = null;
        }
    };

    KitSession.prototype._send = function(packet) {
        var self = this;
        self.socket.send(packet.buffer);
    };

    KitSession.prototype._read = function(raw) {
        var self = this;
        var pkts = Package.decode(raw);
        if(Array.isArray(pkts)) {
            for(var i=0; i<pkts.length; i++) {
                var pkt = pkts[i];
                self._onPacket(pkt);
            }
        } else {
            self._onPacket(pkts);
        }
    };

    KitSession.prototype._onPacket = function(pkt) {
        var self = this;
        switch(pkt.type) {
            case Package.TYPE_HANDSHAKE:
                self._onHandshake(pkt.body);
                break;
            case Package.TYPE_HEARTBEAT:
                self._onHeartbeat(pkt.body);
                break;
            case Package.TYPE_DATA:
                self._onData(pkt.body);
                break;
            case Package.TYPE_KICK:
                self._onKick(pkt.body);
                break;
        }
    };

    KitSession.prototype._onData = function(msg) {
        var self = this;
        msg = Message.decode(msg);
        msg.body = JSON.parse(Protocol.strdecode(msg.body));

        if (!msg.id) {
            self.emit(msg.route, msg.body);
            return;
        }

        var cb = self._requestCallbacks[msg.id];
        if (cb) {
            cb(msg.body);
        }
        delete(self._requestCallbacks[msg.id]);
    };

    KitSession.prototype._onKick = function(msg) {
        var self = this;
        self.log && console.log('session closed by server');
        self.disconnect();
    };

    KitSession.prototype._onHeartbeat = function(msg) {
        var self = this;
        // self.log && console.log('receiv heartbeat');
    };

    KitSession.prototype._setupHeartbeat = function(msg) {
        var self = this;

        if (self._heartbeatTimer) {
            clearInterval(self._heartbeatTimer);
            self._heartbeatTimer = null;
        }

        if (self._heartbeatInterval) {
            self._heartbeatTimer = setInterval(function() {
                // self.log && console.log('send heartbeat');
                var pkt = Package.encode(Package.TYPE_HEARTBEAT);
                self._send(pkt);
            }, self._heartbeatInterval * 1000);
        }
    };

    KitSession.prototype._onHandshake = function(msg) {
        var self = this;
        self.state = KitSession.Open;

        msg = JSON.parse(Protocol.strdecode(msg));
        self.log && console.log('onHandshake', msg);

        if (msg.sid) {
            self.sid = msg.sid;
            self.log && console.log('sid', self.sid);
        } else {
            self.log && console.error('can\'t find sid')
        }

        if (msg.hb) {
            self._heartbeatInterval = msg.hb;
            self._setupHeartbeat();
        }

        var pkt = Package.encode(Package.TYPE_HANDSHAKE_ACK);
        self._send(pkt);

        if (self._delayBuffer) {
            self._delayBuffer.forEach(function(msg) {
                self._send(msg);
            });
            self._delayBuffer = [];
        }

        if (self._reconnectAttempts > 0) {
            self.log && console.log('reconnect success');
            self._reconnectAttempts = 0;
        } else {
            self.log && console.log('session is ready');
            self._readyCb && self._readyCb();
        }
    };

    KitSession.prototype._connect = function() {
        var self = this;
        self.state = KitSession.Connecting

        var socket = self.socket = new WebSocket(this.url);
        socket.binaryType = 'arraybuffer';

        socket.onopen = function(e) {
            var req = {sid:self.sid};
            var obj = Package.encode(Package.TYPE_HANDSHAKE, Protocol.strencode(JSON.stringify(req)));
            self._send(obj);
        };

        function reconnect(e) {
            if (self.state == KitSession.Closed || self.state == KitSession.Closing) {
                return;
            }

            self.log && console.error(e);
            self._close();

            if (self._reconnectMaxAttempts > self._reconnectAttempts) {
                if (self._reconnectTimer) {
                    clearTimeout(self._reconnectTimer);
                }
                self._reconnectTimer = setTimeout(function() {
                    self.log && console.log('reconnect');
                    self._reconnectAttempts ++;
                    self._connect();
                }, self._reconnectDelay * 1000);
            } else {
                self.log && console.log('reconnect failed after '+self._reconnectAttempts+' attempts');
                self.disconnect();
            }
        }

        socket.onerror = reconnect;
        socket.onclose = reconnect;

        socket.onmessage = function(e) {
            self._read(e.data);
        };
    }


    if (Global) {
        Global.KitSession = KitSession;
    }

    if (typeof module !== 'undefined') {
        module.exports = KitSession;
    }
})();
