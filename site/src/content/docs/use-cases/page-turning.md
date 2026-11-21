---
title: Page turning & streaming
description: Turn pages in forScore and OnSong, or drive a teleprompter, with the pedal's keyboard mode.
---

Reading apps and streaming tools usually accept a keyboard, not MIDI. The
Chocolate's **Keyboard B** mode turns the four switches into Page Up, Page
Down, Space and Enter over USB HID — no MIDI mapping, no driver.

## forScore (iPad)

forScore can learn MIDI page turns and also accepts keyboard shortcuts.

1. Connect the pedal over USB to the computer running forScore, or use
   Bluetooth MIDI if forScore sees it.
2. For MIDI: forScore → Settings → **Page Turners & Shortcuts** → learn a turn,
   then step on a switch.
3. For keyboard: put the pedal in **Keyboard B** and check that Page Up/Down
   moves the page.

forScore's MIDI learn has a documented limitation with some range messages;
if a switch only turns one way, use the keyboard mode instead.

Sources: forScore *Page Turners & Shortcuts*.

## OnSong (iOS)

OnSong's foot-pedal support is configured per pedal, and some MIDI features are
part of the paid MIDI Integration. Start with the built-in pedal profiles, then
learn CCs if your mapping differs.

Source: OnSong *Foot Pedals* documentation.

## Teleprompters and OBS

Any tool with keyboard shortcuts works: put the pedal in Keyboard B and map
Page Up / Page Down / Space / Enter to scroll, start or pause. OBS, browser
teleprompters and slide decks all accept those keys.

## A caveat about HID and Bluetooth

The USB connection exposes a real HID keyboard interface in Keyboard modes.
Bluetooth MIDI is a MIDI transport; it does not automatically become a
Bluetooth keyboard. If your tablet only sees the pedal as a MIDI device, use
MIDI learn in the app rather than expecting a Page Down key.

## Which mode to pick

| Target | Mode |
| --- | --- |
| forScore / OnSong with MIDI learn | Custom CC or Program Change |
| Any app with keyboard shortcuts | Keyboard B |
| Custom single keys or combos | Custom Keyboard |
| Media playback | Multimedia or Video (HID) |
