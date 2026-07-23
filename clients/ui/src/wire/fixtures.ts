import controlWithCode from "./testdata/control-with-code.json?raw";
import control from "./testdata/control.json?raw";
import hello from "./testdata/hello.json?raw";
import output from "./testdata/output.json?raw";
import viewUpdate from "./testdata/view-update.json?raw";

export const fixtures = {
  output,
  control,
  controlWithCode,
  hello,
  viewUpdate,
  respOK: '{"k":"r","reqId":"req-1","body":{"ok":true}}',
  respErr: '{"k":"e","reqId":"req-2","code":"frame-not-ready","message":"not yet"}',
  transcriptTail: '{"k":"tt","sessionId":"s1","line":"[claude] hello world"}',
  eventLogTail: '{"k":"et","sessionId":"s1","line":"event line"}',
  notification:
    '{"k":"n","sessionId":"s1","cmd":9,"title":"Task done","body":"Session finished","nowMs":1700000002000}',
} as const;
