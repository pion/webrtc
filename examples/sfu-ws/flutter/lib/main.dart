import 'dart:math';

import 'package:flutter/material.dart';
import 'dart:convert';
import 'dart:async';
import 'dart:io';
import 'package:flutter_webrtc/webrtc.dart';

void main() => runApp(MyApp());

class MyApp extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Flutter Demo',
      theme: ThemeData(
        primarySwatch: Colors.blue,
      ),
      home: MyHomePage(title: 'Flutter Demo Home Page'),
    );
  }
}

class MyHomePage extends StatefulWidget {
  MyHomePage({Key key, this.title}) : super(key: key);

  final String title;

  @override
  _MyHomePageState createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  var _socket;
  var _host = '192.168.1.4';
  MediaStream _stream;
  RTCPeerConnection _pc;
  RTCDataChannel _dc;
  final _renderer = new RTCVideoRenderer();
  bool _inCalling = false;

  Map<String, dynamic> configuration = {
    "iceServers": [
      {"url": "stun:stun.l.google.com:19302"},
    ]
  };

  final Map<String, dynamic> _config = {
    'mandatory': {},
    'optional': [
      {'DtlsSrtpKeyAgreement': true},
    ],
  };

  final Map<String, dynamic> _constraints = {
    'mandatory': {
      'OfferToReceiveAudio': true,
      'OfferToReceiveVideo': true,
    },
    'optional': [],
  };

  @override
  initState() {
    super.initState();
    _renderer.initialize();
  }

  void _onOpen() {
   _createPublisher();
  }

  void _onMessage(data){
    if(_pc == null) return;
    _pc.setRemoteDescription(new RTCSessionDescription(data, 'answer'));
  }

  void _createPublisher() async {
    if (_inCalling) {
      return;
    }

    final Map<String, dynamic> mediaConstraints = {
      "audio": true,
      "video": {
        "mandatory": {
          "minWidth":
              '640', // Provide your own width, height and frame rate here
          "minHeight": '480',
          "minFrameRate": '30',
        },
        "facingMode": "user",
        "optional": [],
      }
    };

    _stream = await navigator.getUserMedia(mediaConstraints);
    _renderer.srcObject = _stream;

    _pc = await createPeerConnection(configuration, _config);
    _dc = await _pc.createDataChannel('data', RTCDataChannelInit());

    _pc.onIceGatheringState = (state) async {
      if(state ==RTCIceGatheringState.RTCIceGatheringStateComplete) {
        print('RTCIceGatheringStateComplete');
        RTCSessionDescription sdp = await _pc.getLocalDescription();
        _send(sdp.sdp);
      }
    };

    _pc.addStream(_stream);

    RTCSessionDescription description =
        await _pc.createOffer(_constraints);
    print('Publisher createOffer');
    _pc.setLocalDescription(description);
    setState(() {
      _inCalling = true;
    });
  }

  void _createSubscriber() async {
   if (_inCalling) {
      return;
    }

    _pc = await createPeerConnection(configuration, _config);

    _dc = await _pc.createDataChannel('data', RTCDataChannelInit());

    _pc.onIceGatheringState = (state) async {
      if(state == RTCIceGatheringState.RTCIceGatheringStateComplete) {
        print('RTCIceGatheringStateComplete');
        RTCSessionDescription sdp = await _pc.getLocalDescription();
        _send(sdp.sdp);
      }
    };

    _pc.onAddStream = (stream) {
      print('Got remote stream => '  + stream.id);
      this.setState((){
      _stream = stream;
      _renderer.srcObject = _stream;
      });
    };

    RTCSessionDescription description =
        await _pc.createOffer(_constraints);
    print('Subscriber createOffer');
    _pc.setLocalDescription(description);
    setState(() {
      _inCalling = true;
    });
  }

  void _connect(String host) async {
    if (_socket != null) {
      print('Already connected!');
      return;
    }
    try {
      Random r = new Random();
      String key = base64.encode(List<int>.generate(8, (_) => r.nextInt(255)));
      SecurityContext securityContext = new SecurityContext();
      HttpClient client = HttpClient(context: securityContext);
      client.badCertificateCallback =
          (X509Certificate cert, String host, int port) {
        print('badCertificateCallback => $host:$port');
        return true;
      };

      HttpClientRequest request = await client.getUrl(
          Uri.parse('https://$host:8443/ws')); // form the correct url here
      request.headers.add('Connection', 'Upgrade');
      request.headers.add('Upgrade', 'websocket');
      request.headers.add(
          'Sec-WebSocket-Version', '13'); // insert the correct version here
      request.headers.add('Sec-WebSocket-Key', key.toLowerCase());

      HttpClientResponse response = await request.close();
      Socket socket = await response.detachSocket();
      _socket = WebSocket.fromUpgradedSocket(
        socket,
        protocol: 'pions-flutter',
        serverSide: false,
      );
      _socket.listen((data) {
        print('Recivied data: ' + data);
        _onMessage(data);
      }, onDone: () {
        print('Closed by server!');
        _socket = null;
      });
      _onOpen();
    } catch (e) {
      print(e.toString());
      _socket = null;
    }
  }

  void _send(String data) {
    if (_socket != null) _socket.add(data);
    print('send: ' + data);
  }

  @override
  deactivate() {
    super.deactivate();
    if (_inCalling) {
      _hangUp();
    }
    _renderer.dispose();
  }

  // Platform messages are asynchronous, so we initialize in an async method.
  _makeCall() async {
    _connect(_host);
  }

  _hangUp() async {
    try {
      await _stream.dispose();
      _renderer.srcObject = null;
      _pc.close();
      _socket.close();
      _socket = null;
    } catch (e) {
      print(e.toString());
    }
    setState(() {
      _inCalling = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return new Scaffold(
      appBar: new AppBar(
        title: new Text('Flutter Pions SFU Test'),
      ),
      body: new OrientationBuilder(
        builder: (context, orientation) {
          return new Center(
            child: new Container(
              margin: new EdgeInsets.fromLTRB(0.0, 0.0, 0.0, 0.0),
              width: MediaQuery.of(context).size.width,
              height: MediaQuery.of(context).size.height,
              child: RTCVideoView(_renderer),
              decoration: new BoxDecoration(color: Colors.black54),
            ),
          );
        },
      ),
      floatingActionButton: new FloatingActionButton(
        onPressed: _inCalling ? _hangUp : _makeCall,
        tooltip: _inCalling ? 'Hangup' : 'Call',
        child: new Icon(_inCalling ? Icons.call_end : Icons.phone),
      ),
    );
  }
}
