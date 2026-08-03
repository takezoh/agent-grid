"use strict";

const { app, screen } = require("electron");

void app.whenReady().then(() => {
  const displays = screen.getAllDisplays().map((display) => {
    const dip = {
      x: Math.round(display.bounds.x + display.bounds.width / 2),
      y: Math.round(display.bounds.y + display.bounds.height / 2),
    };
    return {
      id: display.id,
      label: display.label,
      bounds: display.bounds,
      workArea: display.workArea,
      dip,
      physical: screen.dipToScreenPoint(dip),
    };
  });
  process.stdout.write(JSON.stringify({
    primaryId: screen.getPrimaryDisplay().id,
    displays,
  }));
  app.quit();
});
