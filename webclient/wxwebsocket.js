function WebSocket(url) {
    var self = this;
    var socket = self.socket = wx.connectSocket({url:url});
    socket.onOpen(function() {
        self.onopen && self.onopen();
    });

    socket.onClose(function() {
        self.onclose && self.onclose();
    });

    socket.onError(function() {
        self.onerror && self.onerror();
    });

    socket.onMessage(function(d) {
        self.onmessage && self.onmessage(d);
    });
}

WebSocket.prototype.send = function(buf) {
    var self = this;
    if (self.socket) {
        self.socket.send({data:buf});
    }
};

WebSocket.prototype.close = function() {
    var self = this;
    self.onopen = null;
    self.onclose = null;
    self.onerror = null;
    self.onmessage = null;
    self.socket.onOpen(null);
    self.socket.onClose(null);
    self.socket.onError(null);
    self.socket.onMessage(null);
    self.socket.close();
    self.socket = null;
};

if (typeof module !== 'undefined') {
    module.exports = WebSocket;
}
