import 'package:flutter/material.dart';
import 'package:flutter_webrtc/webrtc.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'sfu_ws_sample.dart';

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

enum PeerType {
  kPublisher,
  kSubscriber,
}

class _MyHomePageState extends State<MyHomePage> {
  final _renderer = new RTCVideoRenderer();
  SfuWsSample _sfuSample;
  String _serverAddress;
  SharedPreferences prefs;
  PeerType _type = PeerType.kPublisher;

  @override
  initState() {
    super.initState();
    init();
  }

  init() async {
    await _renderer.initialize();
    prefs = await SharedPreferences.getInstance();
    setState(() {
      _serverAddress = prefs.getString('server');
    });
  }

  @override
  deactivate() {
    super.deactivate();
    if (_sfuSample != null) {
      _hangUp();
    }
    _renderer.dispose();
  }

  _makeCall() async {
    if (_sfuSample != null) {
      print('Already in calling!');
      return;
    }

    _sfuSample = new SfuWsSample();

    _sfuSample.onOpen = () {
      if (_type == PeerType.kPublisher)
        _sfuSample.createPublisher();
      else if (_type == PeerType.kSubscriber) {
        _sfuSample.createSubscriber();
      }
    };

    _sfuSample.onLocalStream = (stream) {
      this.setState(() {
        _renderer.srcObject = stream;
      });
    };

    _sfuSample.onRemoteStream = (stream) {
      this.setState(() {
        _renderer.srcObject = stream;
      });
    };

    await _sfuSample.connect(_serverAddress);
  }

  _hangUp() async {
    try {
      if (_sfuSample != null) {
        _sfuSample.close();
        _renderer.srcObject = null;
      }
    } catch (e) {
      print(e.toString());
    }
    setState(() {
      _sfuSample = null;
    });
  }

  _buildSetupWidgets(context) {
    return new Align(
        alignment: Alignment(0, 0),
        child: Column(
            crossAxisAlignment: CrossAxisAlignment.center,
            mainAxisAlignment: MainAxisAlignment.center,
            children: <Widget>[
              SizedBox(
                  width: 260.0,
                  child: TextField(
                    keyboardType: TextInputType.text,
                    textAlign: TextAlign.center,
                    decoration: InputDecoration(
                      //内容的内边距
                      contentPadding: EdgeInsets.all(10.0),
                      border: UnderlineInputBorder(
                          borderSide: BorderSide(color: Colors.black12)),
                      hintText: _serverAddress?? 'Enter Pion-SFU address.',
                    ),
                    onChanged: (value) {
                      setState(() {
                        _serverAddress = value;
                      });
                    },
                  )),
              SizedBox(width: 260.0, height: 48.0),
              SizedBox(
                  width: 260.0,
                  height: 48.0,
                  child: Row(
                    children: <Widget>[
                      Radio<PeerType>(
                          value: PeerType.kPublisher,
                          groupValue: _type,
                          onChanged: (value) {
                            setState(() {
                              _type = value;
                            });
                          }),
                      Text('Publisher'),
                      Radio<PeerType>(
                          value: PeerType.kSubscriber,
                          groupValue: _type,
                          onChanged: (value) {
                            setState(() {
                              _type = value;
                            });
                          }),
                      Text('Subscriber'),
                    ],
                  )),
              SizedBox(width: 260.0, height: 48.0),
              SizedBox(
                  width: 220.0,
                  height: 48.0,
                  child: MaterialButton(
                    child: Text(
                      'Connect',
                      style: TextStyle(fontSize: 16.0, color: Colors.white),
                    ),
                    color: Colors.blue,
                    textColor: Colors.white,
                    onPressed: () {
                      if (_serverAddress != null) {
                        _makeCall();
                        prefs.setString('server', _serverAddress);
                        return;
                      }
                      showDialog<Null>(
                        context: context,
                        barrierDismissible: false,
                        builder: (BuildContext context) {
                          return new AlertDialog(
                            title: new Text('Server is empty'),
                            content: new Text('Please enter Pion-SFU address!'),
                            actions: <Widget>[
                              new FlatButton(
                                child: new Text('Ok'),
                                onPressed: () {
                                  Navigator.of(context).pop();
                                },
                              ),
                            ],
                          );
                        },
                      );
                    },
                  ))
            ]));
  }

  _buildCallWidgets(context) {
    return new Center(
      child: new Container(
        margin: new EdgeInsets.fromLTRB(0.0, 0.0, 0.0, 0.0),
        width: MediaQuery.of(context).size.width,
        height: MediaQuery.of(context).size.height,
        child: RTCVideoView(_renderer),
        decoration: new BoxDecoration(color: Colors.black54),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return new Scaffold(
      appBar: new AppBar(
        title: new Text('Flutter Pions SFU Test'),
      ),
      body: new OrientationBuilder(
        builder: (context, orientation) {
          return Container(
              color: Colors.white,
              child: _sfuSample == null
                  ? _buildSetupWidgets(context)
                  : _buildCallWidgets(context));
        },
      ),
      floatingActionButton: _sfuSample == null
          ? null
          : new FloatingActionButton(
              onPressed: _hangUp,
              tooltip: 'Hangup',
              child: new Icon(
                Icons.call_end,
              ),
              backgroundColor: Colors.red,
            ),
    );
  }
}
