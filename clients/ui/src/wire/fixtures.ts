import controlDepartureBare from "./testdata/control-departure-bare.json?raw";
import controlDeparture from "./testdata/control-departure.json?raw";
import controlWithCode from "./testdata/control-with-code.json?raw";
import control from "./testdata/control.json?raw";
import hello from "./testdata/hello.json?raw";
import output from "./testdata/output.json?raw";
import viewUpdate from "./testdata/view-update.json?raw";

export const fixtures = {
  output,
  control,
  controlWithCode,
  // Departure control frames carry the published transport-close reason
  // (protocol/transport-close.schema.json). This client does not read it yet;
  // the fixtures are here so the codec is proven to keep parsing the frame,
  // and to keep matching the `data` literal it does read.
  controlDeparture,
  controlDepartureBare,
  hello,
  viewUpdate,
  respOK: '{"k":"r","reqId":"req-1","body":{"ok":true}}',
  respErr: '{"k":"e","reqId":"req-2","code":"frame-not-ready","message":"not yet"}',
  transcriptTail: '{"k":"tt","sessionId":"s1","line":"[claude] hello world"}',
  eventLogTail: '{"k":"et","sessionId":"s1","line":"event line"}',
  notification:
    '{"k":"n","sessionId":"s1","cmd":9,"title":"Task done","body":"Session finished","nowMs":1700000002000}',
} as const;
