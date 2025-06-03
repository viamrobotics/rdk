package evdev

//go:generate ./gen.sh

// derived from linux/libedev's input.h and input-event-codes.h

// BusType is the device bus type.
type BusType uint16

// BusType values.
const (
	BusPCI          BusType = 0x01
	BusISAPNP       BusType = 0x02
	BusUSB          BusType = 0x03
	BusHIL          BusType = 0x04
	BusBluetooth    BusType = 0x05
	BusVirtual      BusType = 0x06
	BusISA          BusType = 0x10
	BusI8042        BusType = 0x11
	BusXTKBD        BusType = 0x12
	BusRS232        BusType = 0x13
	BusGamePort     BusType = 0x14
	BusParallelPort BusType = 0x15
	BusAmiga        BusType = 0x16
	BusADB          BusType = 0x17
	BusI2C          BusType = 0x18
	BusHost         BusType = 0x19
	BusGSC          BusType = 0x1a
	BusAtari        BusType = 0x1b
	BusSPI          BusType = 0x1c
	BusRMI          BusType = 0x1d
	BusCEC          BusType = 0x1e
	BusIntelISHTP   BusType = 0x1f
)

// EventType is the event type.
type EventType uint16

// Event types.
const (
	EventSync         EventType = 0x00 // Synchronization events.
	EventKey          EventType = 0x01 // Key and button events.
	EventRelative     EventType = 0x02 // Relative axis events (such as a mouse).
	EventAbsolute     EventType = 0x03 // Absolute axis events (such as a joystick).
	EventMisc         EventType = 0x04 // Misc events.
	EventSwitch       EventType = 0x05 // Used to describe stateful binary switches.
	EventLED          EventType = 0x11 // LEDs and similar indications.
	EventSound        EventType = 0x12 // Sound output events.
	EventRepeat       EventType = 0x14 // Repeat key events.
	EventEffect       EventType = 0x15 // Force feedback effect events.
	EventPower        EventType = 0x16 // Power management events.
	EventEffectStatus EventType = 0x17 // Effect status events.

	eventMax = 0x1f
)

// SyncType is the synchronization event type.
//
// Synchronization event values are undefined. Their usage is defined only by
// when they are sent in the evdev event stream.
//
// SyncReport is used to synchronize and separate events into packets of input
// data changes occurring at the same moment in time. For example, motion of a
// mouse may set the RelativeX and RelativeY values for one motion, then emit a
// SyncReport. The next motion will emit more RelativeX and RelativeY values and send
// another SyncReport.
//
// SyncConfig: to be determined.
//
// SyncMTReport is used to synchronize and separate touch events. See the
// multi-touch-protocol.txt document for more information.
//
// SyncDropped is used to indicate buffer overrun in the evdev client's event
// queue. Client should ignore all events up to and including next SyncReport
// event and query the device (using EVIOCG* ioctls) to obtain its current
// state.
type SyncType int

// Sync event types.
const (
	SyncReport     SyncType = 0
	SyncConfig     SyncType = 1
	SyncMTReport   SyncType = 2
	SyncDropped    SyncType = 3
	SyncDisconnect SyncType = 4 //Not from linux, generated when device file disappears

	syncMax = 0xf
)

// KeyType is the key event type.
//
// Events take the form Key<name> or Btn<name>. For example, KeyA is used to
// represent the 'A' key on a keyboard. When a key is depressed, an event with
// the key's code is emitted with value 1. When the key is released, an event
// is emitted with value 0. Some hardware send events when a key is repeated.
// These events have a value of 2. In general, Key<name> is used for keyboard
// keys, and Btn<name> is used for other types of momentary switch events.
//
// A few codes have special meanings:
//
// 	* BtnTool<name>:
// 	    These codes are used in conjunction with input trackpads, tablets, and
// 	    touchscreens. These devices may be used with fingers, pens, or other
// 	    tools. When an event occurs and a tool is used, the corresponding
// 	    BtnTool<name> code should be set to a value of 1. When the tool is no
// 	    longer interacting with the input device, the BtnTool<name> code should
// 	    be reset to 0. All trackpads, tablets, and touchscreens should use at
// 	    least one BtnTool<name> code when events are generated.
//
// 	* BtnTouch:
// 		BtnTouch is used for touch contact. While an input tool is determined
// 		to be within meaningful physical contact, the value of this property
// 		must be set to 1. Meaningful physical contact may mean any contact, or
// 		it may mean contact conditioned by an implementation defined property.
// 		For example, a touchpad may set the value to 1 only when the touch
// 		pressure rises above a certain value. BtnTouch may be combined with
// 		BtnTool<name> codes. For example, a pen tablet may set BtnToolPen to 1
// 		and BtnTouch to 0 while the pen is hovering over but not touching the
// 		tablet surface.
//
// 	* BtnToolFinger, BtnToolDoubleTap, BtnToolTrippleTap, BtnToolQuadTap:
// 	    These codes denote one, two, three, and four finger interaction on a
// 	    trackpad or touchscreen. For example, if the user uses two fingers and
// 	    moves them on the touchpad in an effort to scroll content on screen,
// 	    BtnToolDoubleTap should be set to value 1 for the duration of the
// 	    motion. Note that all BtnTool<name> codes and the BtnTouch code are
// 	    orthogonal in purpose. A trackpad event generated by finger touches
// 	    should generate events for one code from each group. At most only one
// 	    of these BtnTool<name> codes should have a value of 1 during any
// 	    synchronization frame.
type KeyType uint32

// Keys event types.
const (
	KeyReserved          KeyType = 0
	KeyEscape            KeyType = 1
	Key1                 KeyType = 2
	Key2                 KeyType = 3
	Key3                 KeyType = 4
	Key4                 KeyType = 5
	Key5                 KeyType = 6
	Key6                 KeyType = 7
	Key7                 KeyType = 8
	Key8                 KeyType = 9
	Key9                 KeyType = 10
	Key0                 KeyType = 11
	KeyMinus             KeyType = 12
	KeyEqual             KeyType = 13
	KeyBackSpace         KeyType = 14
	KeyTab               KeyType = 15
	KeyQ                 KeyType = 16
	KeyW                 KeyType = 17
	KeyE                 KeyType = 18
	KeyR                 KeyType = 19
	KeyT                 KeyType = 20
	KeyY                 KeyType = 21
	KeyU                 KeyType = 22
	KeyI                 KeyType = 23
	KeyO                 KeyType = 24
	KeyP                 KeyType = 25
	KeyLeftBrace         KeyType = 26
	KeyRightBrace        KeyType = 27
	KeyEnter             KeyType = 28
	KeyLeftCtrl          KeyType = 29
	KeyA                 KeyType = 30
	KeyS                 KeyType = 31
	KeyD                 KeyType = 32
	KeyF                 KeyType = 33
	KeyG                 KeyType = 34
	KeyH                 KeyType = 35
	KeyJ                 KeyType = 36
	KeyK                 KeyType = 37
	KeyL                 KeyType = 38
	KeySemiColon         KeyType = 39
	KeyApostrophe        KeyType = 40
	KeyGrave             KeyType = 41
	KeyLeftShift         KeyType = 42
	KeyBackSlash         KeyType = 43
	KeyZ                 KeyType = 44
	KeyX                 KeyType = 45
	KeyC                 KeyType = 46
	KeyV                 KeyType = 47
	KeyB                 KeyType = 48
	KeyN                 KeyType = 49
	KeyM                 KeyType = 50
	KeyComma             KeyType = 51
	KeyDot               KeyType = 52
	KeySlash             KeyType = 53
	KeyRightShift        KeyType = 54
	KeyKeypadAsterisk    KeyType = 55
	KeyLeftAlt           KeyType = 56
	KeySpace             KeyType = 57
	KeyCapsLock          KeyType = 58
	KeyF1                KeyType = 59
	KeyF2                KeyType = 60
	KeyF3                KeyType = 61
	KeyF4                KeyType = 62
	KeyF5                KeyType = 63
	KeyF6                KeyType = 64
	KeyF7                KeyType = 65
	KeyF8                KeyType = 66
	KeyF9                KeyType = 67
	KeyF10               KeyType = 68
	KeyNumLock           KeyType = 69
	KeyScrollLock        KeyType = 70
	KeyKeypad7           KeyType = 71
	KeyKeypad8           KeyType = 72
	KeyKeypad9           KeyType = 73
	KeyKeypadMinus       KeyType = 74
	KeyKeypad4           KeyType = 75
	KeyKeypad5           KeyType = 76
	KeyKeypad6           KeyType = 77
	KeyKeypadPlus        KeyType = 78
	KeyKeypad1           KeyType = 79
	KeyKeypad2           KeyType = 80
	KeyKeypad3           KeyType = 81
	KeyKeypad0           KeyType = 82
	KeyKeypadDot         KeyType = 83
	KeyZenkakuHankaku    KeyType = 85
	Key102ND             KeyType = 86
	KeyF11               KeyType = 87
	KeyF12               KeyType = 88
	KeyRO                KeyType = 89
	KeyKatakana          KeyType = 90
	KeyHiragana          KeyType = 91
	KeyHenkan            KeyType = 92
	KeyKatakanaHiragana  KeyType = 93
	KeyMuhenkan          KeyType = 94
	KeyKeypadJPComma     KeyType = 95
	KeyKeypadEnter       KeyType = 96
	KeyRightCtrl         KeyType = 97
	KeyKeypadSlash       KeyType = 98
	KeySysRQ             KeyType = 99
	KeyRightAlt          KeyType = 100
	KeyLineFeed          KeyType = 101
	KeyHome              KeyType = 102
	KeyUp                KeyType = 103
	KeyPageUp            KeyType = 104
	KeyLeft              KeyType = 105
	KeyRight             KeyType = 106
	KeyEnd               KeyType = 107
	KeyDown              KeyType = 108
	KeyPageDown          KeyType = 109
	KeyInsert            KeyType = 110
	KeyDelete            KeyType = 111
	KeyMacro             KeyType = 112
	KeyMute              KeyType = 113
	KeyVolumeDown        KeyType = 114
	KeyVolumeUp          KeyType = 115
	KeyPower             KeyType = 116 // SC System Power Down
	KeyKeypadEqual       KeyType = 117
	KeyKeypadPlusMinus   KeyType = 118
	KeyPause             KeyType = 119
	KeyScale             KeyType = 120 // AL Compiz Scale (Expose)
	KeyKeypadComma       KeyType = 121
	KeyHangul            KeyType = 122
	KeyHanja             KeyType = 123
	KeyYen               KeyType = 124
	KeyLeftMeta          KeyType = 125
	KeyRightMeta         KeyType = 126
	KeyCompose           KeyType = 127
	KeyStop              KeyType = 128 // AC Stop
	KeyAgain             KeyType = 129
	KeyProps             KeyType = 130 // AC Properties
	KeyUndo              KeyType = 131 // AC Undo
	KeyFront             KeyType = 132
	KeyCopy              KeyType = 133 // AC Copy
	KeyOpen              KeyType = 134 // AC Open
	KeyPaste             KeyType = 135 // AC Paste
	KeyFind              KeyType = 136 // AC Search
	KeyCut               KeyType = 137 // AC Cut
	KeyHelp              KeyType = 138 // AL Integrated Help Center
	KeyMenu              KeyType = 139 // Menu (show menu)
	KeyCalc              KeyType = 140 // AL Calculator
	KeySetup             KeyType = 141
	KeySleep             KeyType = 142 // SC System Sleep
	KeyWakeup            KeyType = 143 // System Wake Up
	KeyFile              KeyType = 144 // AL Local Machine Browser
	KeySendFile          KeyType = 145
	KeyDeleteFile        KeyType = 146
	KeyXfer              KeyType = 147
	KeyProg1             KeyType = 148
	KeyProg2             KeyType = 149
	KeyWWW               KeyType = 150 // AL Internet Browser
	KeyMSDOS             KeyType = 151
	KeyScreenlock        KeyType = 152
	KeyDirection         KeyType = 153
	KeyCycleWindows      KeyType = 154
	KeyMail              KeyType = 155
	KeyBookmarks         KeyType = 156 // AC Bookmarks
	KeyComputer          KeyType = 157
	KeyBack              KeyType = 158 // AC Back
	KeyForward           KeyType = 159 // AC Forward
	KeyCloseCD           KeyType = 160
	KeyEjectCD           KeyType = 161
	KeyEjectCloseCD      KeyType = 162
	KeyNextSong          KeyType = 163
	KeyPlayPause         KeyType = 164
	KeyPreviousSong      KeyType = 165
	KeyStopCD            KeyType = 166
	KeyRecord            KeyType = 167
	KeyRewind            KeyType = 168
	KeyPhone             KeyType = 169 // Media Select Telephone
	KeyISO               KeyType = 170
	KeyConfig            KeyType = 171 // AL Consumer Control Configuration
	KeyHomepage          KeyType = 172 // AC Home
	KeyRefresh           KeyType = 173 // AC Refresh
	KeyExit              KeyType = 174 // AC Exit
	KeyMove              KeyType = 175
	KeyEdit              KeyType = 176
	KeyScrollUp          KeyType = 177
	KeyScrollDown        KeyType = 178
	KeyKeypadLeftParen   KeyType = 179
	KeyKeypadRightParen  KeyType = 180
	KeyNew               KeyType = 181 // AC New
	KeyRedo              KeyType = 182 // AC Redo/Repeat
	KeyF13               KeyType = 183
	KeyF14               KeyType = 184
	KeyF15               KeyType = 185
	KeyF16               KeyType = 186
	KeyF17               KeyType = 187
	KeyF18               KeyType = 188
	KeyF19               KeyType = 189
	KeyF20               KeyType = 190
	KeyF21               KeyType = 191
	KeyF22               KeyType = 192
	KeyF23               KeyType = 193
	KeyF24               KeyType = 194
	KeyPlayCD            KeyType = 200
	KeyPauseCD           KeyType = 201
	KeyProg3             KeyType = 202
	KeyProg4             KeyType = 203
	KeyDashboard         KeyType = 204 // AL Dashboard
	KeySuspend           KeyType = 205
	KeyClose             KeyType = 206 // AC Close
	KeyPlay              KeyType = 207
	KeyFastForward       KeyType = 208
	KeyBassBoost         KeyType = 209
	KeyPrint             KeyType = 210 // AC Print
	KeyHP                KeyType = 211
	KeyCamera            KeyType = 212
	KeySound             KeyType = 213
	KeyQuestion          KeyType = 214
	KeyEmail             KeyType = 215
	KeyChat              KeyType = 216
	KeySearch            KeyType = 217
	KeyConnect           KeyType = 218
	KeyFinance           KeyType = 219 // AL Checkbook/Finance
	KeySport             KeyType = 220
	KeyShop              KeyType = 221
	KeyAltErase          KeyType = 222
	KeyCancel            KeyType = 223 // AC Cancel
	KeyBrightnessDown    KeyType = 224
	KeyBrightnessUp      KeyType = 225
	KeyMedia             KeyType = 226
	KeySwitchVideoMode   KeyType = 227 // Cycle between available video  outputs (Monitor/LCD/TV-out/etc)
	KeyKbdIllumToggle    KeyType = 228
	KeyKbdIllumDown      KeyType = 229
	KeyKbdIllumUp        KeyType = 230
	KeySend              KeyType = 231 // AC Send
	KeyReply             KeyType = 232 // AC Reply
	KeyForwardMail       KeyType = 233 // AC Forward Msg
	KeySave              KeyType = 234 // AC Save
	KeyDocuments         KeyType = 235
	KeyBattery           KeyType = 236
	KeyBluetooth         KeyType = 237
	KeyWLAN              KeyType = 238
	KeyUWB               KeyType = 239
	KeyUnknown           KeyType = 240
	KeyVideoNext         KeyType = 241 // drive next video source
	KeyVideoPrevious     KeyType = 242 // drive previous video source
	KeyBrightnessCycle   KeyType = 243 // brightness up, after max is min
	KeyBrightnessZero    KeyType = 244 // brightness off, use ambient
	KeyDisplayOff        KeyType = 245 // display device to off state
	KeyWiMax             KeyType = 246
	KeyRFKill            KeyType = 247 // Key that controls all radios
	KeyMicMute           KeyType = 248 // Mute / unmute the microphone
	KeyOk                KeyType = 0x160
	KeySelect            KeyType = 0x161
	KeyGoto              KeyType = 0x162
	KeyClear             KeyType = 0x163
	KeyPower2            KeyType = 0x164
	KeyOption            KeyType = 0x165
	KeyInfo              KeyType = 0x166 // AL OEM Features/Tips/Tutorial
	KeyTime              KeyType = 0x167
	KeyVendor            KeyType = 0x168
	KeyArchive           KeyType = 0x169
	KeyProgram           KeyType = 0x16a // Media Select Program Guide
	KeyChannel           KeyType = 0x16b
	KeyFavorites         KeyType = 0x16c
	KeyEPG               KeyType = 0x16d
	KeyPVR               KeyType = 0x16e // Media Select Home
	KeyMHP               KeyType = 0x16f
	KeyLanguage          KeyType = 0x170
	KeyTitle             KeyType = 0x171
	KeySubtitle          KeyType = 0x172
	KeyAngle             KeyType = 0x173
	KeyZoom              KeyType = 0x174
	KeyMode              KeyType = 0x175
	KeyKeyboard          KeyType = 0x176
	KeyScreen            KeyType = 0x177
	KeyPC                KeyType = 0x178 // Media Select Computer
	KeyTV                KeyType = 0x179 // Media Select TV
	KeyTV2               KeyType = 0x17a // Media Select Cable
	KeyVCR               KeyType = 0x17b // Media Select VCR
	KeyVCR2              KeyType = 0x17c // VCR Plus
	KeySAT               KeyType = 0x17d // Media Select Satellite
	KeySAT2              KeyType = 0x17e
	KeyCD                KeyType = 0x17f // Media Select CD
	KeyTape              KeyType = 0x180 // Media Select Tape
	KeyRadio             KeyType = 0x181
	KeyTuner             KeyType = 0x182 // Media Select Tuner
	KeyPlayer            KeyType = 0x183
	KeyText              KeyType = 0x184
	KeyDVD               KeyType = 0x185 // Media Select DVD
	KeyAUX               KeyType = 0x186
	KeyMP3               KeyType = 0x187
	KeyAudio             KeyType = 0x188 // AL Audio Browser
	KeyVideo             KeyType = 0x189 // AL Movie Browser
	KeyDirectory         KeyType = 0x18a
	KeyList              KeyType = 0x18b
	KeyMemo              KeyType = 0x18c // Media Select Messages
	KeyCalender          KeyType = 0x18d
	KeyRed               KeyType = 0x18e
	KeyGreen             KeyType = 0x18f
	KeyYellow            KeyType = 0x190
	KeyBlue              KeyType = 0x191
	KeyChannelUp         KeyType = 0x192 // Channel Increment
	KeyChannelDown       KeyType = 0x193 // Channel Decrement
	KeyFirst             KeyType = 0x194
	KeyLast              KeyType = 0x195 // Recall Last
	KeyAB                KeyType = 0x196
	KeyNext              KeyType = 0x197
	KeyRestart           KeyType = 0x198
	KeySlow              KeyType = 0x199
	KeyShuffle           KeyType = 0x19a
	KeyBreak             KeyType = 0x19b
	KeyPrevious          KeyType = 0x19c
	KeyDigits            KeyType = 0x19d
	KeyTeen              KeyType = 0x19e
	KeyTwen              KeyType = 0x19f
	KeyVideoPhone        KeyType = 0x1a0 // Media Select Video Phone
	KeyGames             KeyType = 0x1a1 // Media Select Games
	KeyZoomIn            KeyType = 0x1a2 // AC Zoom In
	KeyZoomOut           KeyType = 0x1a3 // AC Zoom Out
	KeyZoomReset         KeyType = 0x1a4 // AC Zoom
	KeyWordProcessor     KeyType = 0x1a5 // AL Word Processor
	KeyEditor            KeyType = 0x1a6 // AL Text Editor
	KeySpreadsheet       KeyType = 0x1a7 // AL Spreadsheet
	KeyGraphicsEditor    KeyType = 0x1a8 // AL Graphics Editor
	KeyPresentation      KeyType = 0x1a9 // AL Presentation App
	KeyDatabase          KeyType = 0x1aa // AL Database App
	KeyNews              KeyType = 0x1ab // AL Newsreader
	KeyVoiceMail         KeyType = 0x1ac // AL Voicemail
	KeyAddressBook       KeyType = 0x1ad // AL Contacts/Address Book
	KeyMessenger         KeyType = 0x1ae // AL Instant Messaging
	KeyDisplayToggle     KeyType = 0x1af // Turn display (LCD) on and off
	KeySpellCheck        KeyType = 0x1b0 // AL Spell Check
	KeyLogoff            KeyType = 0x1b1 // AL Logoff
	KeyDollar            KeyType = 0x1b2
	KeyEuro              KeyType = 0x1b3
	KeyFrameBack         KeyType = 0x1b4 // Consumer - transport controls
	KeyframeForward      KeyType = 0x1b5
	KeyContextMenu       KeyType = 0x1b6 // GenDesc - system context menu
	KeyMediaRepeat       KeyType = 0x1b7 // Consumer - transport control
	Key10ChannelsUp      KeyType = 0x1b8 // 10 channels up (10+)
	Key10ChannelsDown    KeyType = 0x1b9 // 10 channels down (10-)
	KeyImages            KeyType = 0x1ba // AL Image Browser
	KeyDelEOL            KeyType = 0x1c0
	KeyDelEOS            KeyType = 0x1c1
	KeyInsLine           KeyType = 0x1c2
	KeyDelLine           KeyType = 0x1c3
	KeyFunc              KeyType = 0x1d0
	KeyFuncEsc           KeyType = 0x1d1
	KeyFuncF1            KeyType = 0x1d2
	KeyFuncF2            KeyType = 0x1d3
	KeyFuncF3            KeyType = 0x1d4
	KeyFuncF4            KeyType = 0x1d5
	KeyFuncF5            KeyType = 0x1d6
	KeyFuncF6            KeyType = 0x1d7
	KeyFuncF7            KeyType = 0x1d8
	KeyFuncF8            KeyType = 0x1d9
	KeyFuncF9            KeyType = 0x1da
	KeyFuncF10           KeyType = 0x1db
	KeyFuncF11           KeyType = 0x1dc
	KeyFuncF12           KeyType = 0x1dd
	KeyFunc1             KeyType = 0x1de
	KeyFunc2             KeyType = 0x1df
	KeyFuncD             KeyType = 0x1e0
	KeyFuncE             KeyType = 0x1e1
	KeyFuncF             KeyType = 0x1e2
	KeyFuncS             KeyType = 0x1e3
	KeyFuncB             KeyType = 0x1e4
	KeyBrailleDot1       KeyType = 0x1f1
	KeyBrailleDot2       KeyType = 0x1f2
	KeyBrailleDot3       KeyType = 0x1f3
	KeyBrailleDot4       KeyType = 0x1f4
	KeyBrailleDot5       KeyType = 0x1f5
	KeyBrailleDot6       KeyType = 0x1f6
	KeyBrailleDot7       KeyType = 0x1f7
	KeyBrailleDot8       KeyType = 0x1f8
	KeyBrailleDot9       KeyType = 0x1f9
	KeyBrailleDot10      KeyType = 0x1fa
	KeyNumeric0          KeyType = 0x200 // used by phones, remote controls,
	KeyNumeric1          KeyType = 0x201 // and other keypads
	KeyNumeric2          KeyType = 0x202
	KeyNumeric3          KeyType = 0x203
	KeyNumeric4          KeyType = 0x204
	KeyNumeric5          KeyType = 0x205
	KeyNumeric6          KeyType = 0x206
	KeyNumeric7          KeyType = 0x207
	KeyNumeric8          KeyType = 0x208
	KeyNumeric9          KeyType = 0x209
	KeyNumericStar       KeyType = 0x20a
	KeyNumericPound      KeyType = 0x20b
	KeyNumericA          KeyType = 0x20c // Phone key A - HUT Telephony 0xb9
	KeyNumericB          KeyType = 0x20d
	KeyNumericC          KeyType = 0x20e
	KeyNumericD          KeyType = 0x20f
	KeyCameraFocus       KeyType = 0x210
	KeyWPSButton         KeyType = 0x211 // WiFi Protected Setup key
	KeyTouchpadToggle    KeyType = 0x212 // Request switch touchpad on or off
	KeyTouchpadOn        KeyType = 0x213
	KeyTouchpadOff       KeyType = 0x214
	KeyCameraZoomIn      KeyType = 0x215
	KeyCameraZoomOut     KeyType = 0x216
	KeyCameraUp          KeyType = 0x217
	KeyCameraDown        KeyType = 0x218
	KeyCameraLeft        KeyType = 0x219
	KeyCameraRight       KeyType = 0x21a
	KeyAttendantOn       KeyType = 0x21b
	KeyAttendantOff      KeyType = 0x21c
	KeyAttendantToggle   KeyType = 0x21d // Attendant call on or off
	KeyLightsToggle      KeyType = 0x21e // Reading light on or off
	KeyAlsToggle         KeyType = 0x230 // Ambient light sensor
	KeyButtonConfig      KeyType = 0x240 // AL Button Configuration
	KeyTaskManager       KeyType = 0x241 // AL Task/Project Manager
	KeyJournal           KeyType = 0x242 // AL Log/Journal/Timecard
	KeyControlPanel      KeyType = 0x243 // AL Control Panel
	KeyAppSelect         KeyType = 0x244 // AL Select Task/Application
	KeyScreensaver       KeyType = 0x245 // AL Screen Saver
	KeyVoiceCommand      KeyType = 0x246 // Listening Voice Command
	KeyAssistant         KeyType = 0x247 // AL Context-aware desktop assistant
	KeyBrightnessMin     KeyType = 0x250 // Set Brightness to Minimum
	KeyBrightnessMax     KeyType = 0x251 // Set Brightness to Maximum
	KeyKbdInputPrev      KeyType = 0x260
	KeyKbdInputNext      KeyType = 0x261
	KeyKbdInputPrevGroup KeyType = 0x262
	KeyKbdInputNextGroup KeyType = 0x263
	KeyKbdInputAccept    KeyType = 0x264
	KeyKbdInputCancel    KeyType = 0x265
	KeyRightUp           KeyType = 0x266
	KeyRightDown         KeyType = 0x267
	KeyLeftUp            KeyType = 0x268
	KeyLeftDown          KeyType = 0x269
	KeyRootMenu          KeyType = 0x26a // Show Device's Root Menu
	KeyMediaTopMenu      KeyType = 0x26b
	KeyNumeric11         KeyType = 0x26c
	KeyNumeric12         KeyType = 0x26d
	KeyAudioDesc         KeyType = 0x26e
	Key3dMode            KeyType = 0x26f
	KeyNextFavorite      KeyType = 0x270
	KeyStopRecord        KeyType = 0x271
	KeyPauseRecord       KeyType = 0x272

	keyMax = 0x2ff
)

// Mouse and gamepad buttons event types.
const (
	Btn0    KeyType = 0x100
	Btn1    KeyType = 0x101
	Btn2    KeyType = 0x102
	Btn3    KeyType = 0x103
	Btn4    KeyType = 0x104
	Btn5    KeyType = 0x105
	Btn6    KeyType = 0x106
	Btn7    KeyType = 0x107
	Btn8    KeyType = 0x108
	Btn9    KeyType = 0x109
	BtnMisc KeyType = 0x100

	BtnLeft    KeyType = 0x110
	BtnRight   KeyType = 0x111
	BtnMiddle  KeyType = 0x112
	BtnSide    KeyType = 0x113
	BtnExtra   KeyType = 0x114
	BtnForward KeyType = 0x115
	BtnBack    KeyType = 0x116
	BtnTask    KeyType = 0x117
	BtnMouse   KeyType = 0x110

	BtnTrigger  KeyType = 0x120
	BtnThumb    KeyType = 0x121
	BtnThumb2   KeyType = 0x122
	BtnTop      KeyType = 0x123
	BtnTop2     KeyType = 0x124
	BtnPinkie   KeyType = 0x125
	BtnBase     KeyType = 0x126
	BtnBase2    KeyType = 0x127
	BtnBase3    KeyType = 0x128
	BtnBase4    KeyType = 0x129
	BtnBase5    KeyType = 0x12a
	BtnBase6    KeyType = 0x12b
	BtnDead     KeyType = 0x12f
	BtnJoystick KeyType = 0x120

	BtnA       KeyType = 0x130
	BtnB       KeyType = 0x131
	BtnC       KeyType = 0x132
	BtnX       KeyType = 0x133
	BtnY       KeyType = 0x134
	BtnZ       KeyType = 0x135
	BtnTL      KeyType = 0x136
	BtnTR      KeyType = 0x137
	BtnTL2     KeyType = 0x138
	BtnTR2     KeyType = 0x139
	BtnSelect  KeyType = 0x13a
	BtnStart   KeyType = 0x13b
	BtnMode    KeyType = 0x13c
	BtnThumbL  KeyType = 0x13d
	BtnThumbR  KeyType = 0x13e
	BtnGamepad KeyType = 0x130

	BtnToolPen        KeyType = 0x140
	BtnTooLRubber     KeyType = 0x141
	BtnToolBrush      KeyType = 0x142
	BtnToolPencil     KeyType = 0x143
	BtnToolAirbrush   KeyType = 0x144
	BtnToolFinger     KeyType = 0x145
	BtnToolMouse      KeyType = 0x146
	BtnToolLens       KeyType = 0x147
	BtnToolQuintTap   KeyType = 0x148 // Five fingers on trackpad
	BtnTouch          KeyType = 0x14a
	BtnStylus         KeyType = 0x14b
	BtnStylus2        KeyType = 0x14c
	BtnToolDoubleTap  KeyType = 0x14d
	BtnToolTrippleTap KeyType = 0x14e
	BtnToolQuadTap    KeyType = 0x14f // Four fingers on trackpad
	BtnDigi           KeyType = 0x140

	BtnGearDown KeyType = 0x150
	BtnGearUp   KeyType = 0x151
	BtnWheel    KeyType = 0x150

	BtnTriggerHappy1  KeyType = 0x2c0
	BtnTriggerHappy2  KeyType = 0x2c1
	BtnTriggerHappy3  KeyType = 0x2c2
	BtnTriggerHappy4  KeyType = 0x2c3
	BtnTriggerHappy5  KeyType = 0x2c4
	BtnTriggerHappy6  KeyType = 0x2c5
	BtnTriggerHappy7  KeyType = 0x2c6
	BtnTriggerHappy8  KeyType = 0x2c7
	BtnTriggerHappy9  KeyType = 0x2c8
	BtnTriggerHappy10 KeyType = 0x2c9
	BtnTriggerHappy11 KeyType = 0x2ca
	BtnTriggerHappy12 KeyType = 0x2cb
	BtnTriggerHappy13 KeyType = 0x2cc
	BtnTriggerHappy14 KeyType = 0x2cd
	BtnTriggerHappy15 KeyType = 0x2ce
	BtnTriggerHappy16 KeyType = 0x2cf
	BtnTriggerHappy17 KeyType = 0x2d0
	BtnTriggerHappy18 KeyType = 0x2d1
	BtnTriggerHappy19 KeyType = 0x2d2
	BtnTriggerHappy20 KeyType = 0x2d3
	BtnTriggerHappy21 KeyType = 0x2d4
	BtnTriggerHappy22 KeyType = 0x2d5
	BtnTriggerHappy23 KeyType = 0x2d6
	BtnTriggerHappy24 KeyType = 0x2d7
	BtnTriggerHappy25 KeyType = 0x2d8
	BtnTriggerHappy26 KeyType = 0x2d9
	BtnTriggerHappy27 KeyType = 0x2da
	BtnTriggerHappy28 KeyType = 0x2db
	BtnTriggerHappy29 KeyType = 0x2dc
	BtnTriggerHappy30 KeyType = 0x2dd
	BtnTriggerHappy31 KeyType = 0x2de
	BtnTriggerHappy32 KeyType = 0x2df
	BtnTriggerHappy33 KeyType = 0x2e0
	BtnTriggerHappy34 KeyType = 0x2e1
	BtnTriggerHappy35 KeyType = 0x2e2
	BtnTriggerHappy36 KeyType = 0x2e3
	BtnTriggerHappy37 KeyType = 0x2e4
	BtnTriggerHappy38 KeyType = 0x2e5
	BtnTriggerHappy39 KeyType = 0x2e6
	BtnTriggerHappy40 KeyType = 0x2e7
	BtnTriggerHappy   KeyType = 0x2c0
)

// RelativeType is the relative axis event type.
//
// Relative events describe relative changes in a property. For example, a
// mouse may move to the left by a certain number of units, but its absolute
// position in space is unknown. If the absolute position is known,
// EventAbsolute codes should be used instead of EventRelative codes.
//
// RelativeWheel and RelativeHWheel are used for vertical and horizontal scroll wheels,
// respectively.
type RelativeType int

// Relative axis event types.
const (
	RelativeX      RelativeType = 0x00
	RelativeY      RelativeType = 0x01
	RelativeZ      RelativeType = 0x02
	RelativeRX     RelativeType = 0x03
	RelativeRY     RelativeType = 0x04
	RelativeRZ     RelativeType = 0x05
	RelativeHWheel RelativeType = 0x06
	RelativeDial   RelativeType = 0x07
	RelativeWheel  RelativeType = 0x08
	RelativeMisc   RelativeType = 0x09

	relativeMax = 0x0f
)

// AbsoluteType is the absolute axis event type.
//
// Absolute events describe absolute changes in a property. For example, a
// touchpad may emit coordinates for a touch location. A few codes have special
// meanings:
//
// AbsoluteDistance is used to describe the distance of a tool from an interaction
// surface. This event should only be emitted while the tool is hovering,
// meaning in close proximity to the device and while the value of the BtnTouch
// code is 0. If the input device may be used freely in three dimensions,
// consider AbsoluteZ instead.
//
// AbsoluteMT<name> is used to describe multitouch input events.
type AbsoluteType int

// Absolute axis event types.
const (
	AbsoluteX             AbsoluteType = 0x00
	AbsoluteY             AbsoluteType = 0x01
	AbsoluteZ             AbsoluteType = 0x02
	AbsoluteRX            AbsoluteType = 0x03
	AbsoluteRY            AbsoluteType = 0x04
	AbsoluteRZ            AbsoluteType = 0x05
	AbsoluteThrottle      AbsoluteType = 0x06
	AbsoluteRudder        AbsoluteType = 0x07
	AbsoluteWheel         AbsoluteType = 0x08
	AbsoluteGas           AbsoluteType = 0x09
	AbsoluteBrake         AbsoluteType = 0x0a
	AbsoluteHat0X         AbsoluteType = 0x10
	AbsoluteHat0Y         AbsoluteType = 0x11
	AbsoluteHat1X         AbsoluteType = 0x12
	AbsoluteHat1Y         AbsoluteType = 0x13
	AbsoluteHat2X         AbsoluteType = 0x14
	AbsoluteHat2Y         AbsoluteType = 0x15
	AbsoluteHat3X         AbsoluteType = 0x16
	AbsoluteHat3Y         AbsoluteType = 0x17
	AbsolutePressure      AbsoluteType = 0x18
	AbsoluteDistance      AbsoluteType = 0x19
	AbsoluteTiltX         AbsoluteType = 0x1a
	AbsoluteTiltY         AbsoluteType = 0x1b
	AbsoluteToolWidth     AbsoluteType = 0x1c
	AbsoluteVolume        AbsoluteType = 0x20
	AbsoluteMisc          AbsoluteType = 0x28
	AbsoluteMTSlot        AbsoluteType = 0x2f // MT slot being modified
	AbsoluteMTTouchMajor  AbsoluteType = 0x30 // Major axis of touching ellipse
	AbsoluteMTTouchMinor  AbsoluteType = 0x31 // Minor axis (omit if circular)
	AbsoluteMTWidthMajor  AbsoluteType = 0x32 // Major axis of approaching ellipse
	AbsoluteMTWidthMinor  AbsoluteType = 0x33 // Minor axis (omit if circular)
	AbsoluteMTOrientation AbsoluteType = 0x34 // Ellipse orientation
	AbsoluteMTPositionX   AbsoluteType = 0x35 // Center X touch position
	AbsoluteMTPositionY   AbsoluteType = 0x36 // Center Y touch position
	AbsoluteMTToolType    AbsoluteType = 0x37 // Type of touching device
	AbsoluteMTBlobID      AbsoluteType = 0x38 // Group a set of packets as a blob
	AbsoluteMTTrackingID  AbsoluteType = 0x39 // Unique ID of initiated contact
	AbsoluteMTPressure    AbsoluteType = 0x3a // Pressure on contact area
	AbsoluteMTDistance    AbsoluteType = 0x3b // Contact hover distance
	AbsoluteMTToolX       AbsoluteType = 0x3c // Center X tool position
	AbsoluteMTToolY       AbsoluteType = 0x3d // Center Y tool position

	absoluteMax = 0x3f
)

// MiscType is the used for other event types that do not fall under other
// categories.
//
// MiscTimestamp has a special meaning. It is used to report the number of
// microseconds since the last reset. This event should be coded as an uint32
// value, which is allowed to wrap around with no special consequence. It is
// assumed that the time difference between two consecutive events is reliable
// on a reasonable time scale (hours). A reset to zero can happen, in which
// case the time since the last event is unknown. If the device does not
// provide this information, the driver must not provide it to user space.
type MiscType int

// Misc event types.
const (
	MiscSerial    MiscType = 0x00
	MiscPulseLED  MiscType = 0x01
	MiscGesture   MiscType = 0x02
	MiscRaw       MiscType = 0x03
	MiscScan      MiscType = 0x04
	MiscTimestamp MiscType = 0x05

	miscMax = 0x07
)

// SwitchType is the switch event type.
//
// Switch events describe stateful binary switches. For example, the SwitchLid code
// is used to denote when a laptop lid is closed.
//
// Upon binding to a device or resuming from suspend, a driver must report the
// current switch state. This ensures that the device, kernel, and userspace
// state is in sync.
//
// Upon resume, if the switch state is the same as before suspend, then the
// input subsystem will filter out the duplicate switch state reports. The
// driver does not need to keep the state of the switch at any time.
type SwitchType int

// Switch types.
const (
	SwitchLid                SwitchType = 0x00 // lid shut
	SwitchTabletMode         SwitchType = 0x01 // tablet mode
	SwitchHeadphoneInsert    SwitchType = 0x02 // inserted
	SwitchRFKillAll          SwitchType = 0x03 // rfkill master switch, type "any"; radio enabled
	SwitchMicrophoneInsert   SwitchType = 0x04 // inserted
	SwitchDock               SwitchType = 0x05 // plugged into dock
	SwitchLineoutInsert      SwitchType = 0x06 // inserted
	SwitchJackPhysicalInsert SwitchType = 0x07 // mechanical switch set
	SwitchVideoOutInsert     SwitchType = 0x08 // inserted
	SwitchCameraLensCover    SwitchType = 0x09 // lens covered
	SwitchKeypadSlide        SwitchType = 0x0a // keypad slide out
	SwitchFrontProximity     SwitchType = 0x0b // front proximity sensor active
	SwitchRotateLock         SwitchType = 0x0c // rotate locked/disabled
	SwitchLineInInsert       SwitchType = 0x0d // inserted

	switchMax = 0x0f
)

// LEDType is the type used to set and query the state of a device's LEDs.
type LEDType int

// LEDs.
const (
	LEDNumLock    LEDType = 0x00
	LEDCapsLock   LEDType = 0x01
	LEDScrollLock LEDType = 0x02
	LEDCompose    LEDType = 0x03
	LEDKana       LEDType = 0x04
	LEDSleep      LEDType = 0x05
	LEDSuspend    LEDType = 0x06
	LEDMute       LEDType = 0x07
	LEDMisc       LEDType = 0x08
	LEDMail       LEDType = 0x09
	LEDCharging   LEDType = 0x0a

	ledMax = 0x0f
)

// SoundType events are used for sending sound commands to simple sound output
// devices.
type SoundType int

// Sound event types.
const (
	SoundClick SoundType = 0x00
	SoundBell  SoundType = 0x01
	SoundTone  SoundType = 0x02

	soundMax = 0x07
)

// RepeatType is the repeat event type.
//
// Repeat events are used for specifying autorepeating events.
type RepeatType int

// Repeat event types.
const (
	RepeatDelay  RepeatType = 0x00
	RepeatPeriod RepeatType = 0x01

	//repeatMax = 0x01
)

// EffectType is the force feedback effect type.
type EffectType int

// Force feedback effect event types.
const (
	EffectRumble   EffectType = 0x50
	EffectPeriodic EffectType = 0x51
	EffectConstant EffectType = 0x52
	EffectSpring   EffectType = 0x53
	EffectFriction EffectType = 0x54
	EffectDamper   EffectType = 0x55
	EffectInertia  EffectType = 0x56
	EffectRamp     EffectType = 0x57

	// periodic effect types
	EffectSquare   EffectType = 0x58
	EffectTriangle EffectType = 0x59
	EffectSine     EffectType = 0x5a
	EffectSawUp    EffectType = 0x5b
	EffectSawDown  EffectType = 0x5c
	EffectCustom   EffectType = 0x5d

	//effectMin         = EffectRumble
	effectMax = EffectRamp
	//effectWaveformMin = EffectSquare
	//effectWaveformMax = EffectCustom
)

// EffectPropType is the force feedback effect property type.
type EffectPropType int

// Force feedback effect event types.
const (
	EffectPropGain       EffectPropType = 0x60
	EffectPropAutoCenter EffectPropType = 0x61

	effectPropMax = 0x7f
)

// EffectDirType is the force feedback effect direction type.
type EffectDirType uint16

// Force feedback effect directions.
const (
	EffectDirDown  EffectDirType = 0x0000 // 0 degrees
	EffectDirLeft  EffectDirType = 0x4000 // 90 degrees
	EffectDirUp    EffectDirType = 0x8000 // 180 degrees
	EffectDirRight EffectDirType = 0xc000 // 270 degrees
)

// EffectStatusType is the force feedback effect status event type.
type EffectStatusType int

// Force feedback status event values.
const (
	EffectStatusStopped EffectStatusType = 0x00
	EffectStatusPlaying EffectStatusType = 0x01

	effectStatusMax = 0x01
)

// PowerType is the power event type.
//
// Power events are a special type of event used specifically for power
// mangement. Its usage is not well defined. To be addressed later.
//
// NOTE: these values have been invented for this package. They're not used,
// nor are they defined anywhere.
type PowerType int

// Power types.
const (
	PowerOff     PowerType = 0x00
	PowerOn      PowerType = 0x01
	PowerStandby PowerType = 0x02

	powerMax = 0x0f
)

// PropertyType is the input device property type, for determining input
// device properties and quirks.
//
// Normally, userspace sets up an input device based on the data it emits,
// i.e., the event types. In the case of two devices emitting the same event
// types, additional information can be provided in the form of device
// properties.
//
// The PropertyPointer property indicates that the device is not transposed on
// the screen and thus requires use of an on-screen pointer to trace user's
// movements. Typical pointer devices: touchpads, tablets, mice; non-pointer
// device: touchscreen.
//
// If neither PropertyDirect or PropertyPointer are set, the property is
// considered undefined and the device type should be deduced in the
// traditional way, using emitted event types.
//
// The PropertyDirect property indicates that device coordinates should be
// directly mapped to screen coordinates (not taking into account trivial
// transformations, such as scaling, flipping and rotating). Non-direct input
// devices require non-trivial transformation, such as absolute to relative
// transformation for touchpads. Typical direct input devices: touchscreens,
// drawing tablets; non-direct devices: touchpads, mice.
//
// If neither PropertyDirect or PropertyPointer are set, the property is
// considered undefined and the device type should be deduced in the
// traditional way, using emitted event types.
//
// For touchpads where the button is placed beneath the surface, such that
// pressing down on the pad causes a button click, this property should be set.
// Common in clickpad notebooks and macbooks from 2009 and onwards.
//
// Originally, the buttonpad property was coded into the bcm5974 driver version
// field under the name integrated button. For backwards compatibility, both
// methods need to be checked in userspace.
//
// Some touchpads, most common between 2008 and 2011, can detect the presence
// of multiple contacts without resolving the individual positions; only the
// number of contacts and a rectangular shape is known.  For such touchpads,
// the semi-mt property should be set.
//
// Depending on the device, the rectangle may enclose all touches, like a
// bounding box, or just some of them, for instance the two most recent
// touches. The diversity makes the rectangle of limited use, but some gestures
// can normally be extracted from it.
//
// If PropertySemiMT is not set, the device is assumed to be a true
// multi-touch device.
type PropertyType int

// Input property types.
const (
	PropertyPointer       PropertyType = 0x00
	PropertyDirect        PropertyType = 0x01
	PropertyButtonPad     PropertyType = 0x02
	PropertySemiMT        PropertyType = 0x03
	PropertyTopButtonPad  PropertyType = 0x04
	PropertyPointingStick PropertyType = 0x05
	PropertyAccelerometer PropertyType = 0x06

	propertyMax = 0x1f
)

// MTToolType is the multitouch tool type.
type MTToolType int

// Multitouch tool values.
const (
	MTToolFinger MTToolType = 0
	MTToolPen    MTToolType = 1

	mtToolMax = MTToolPen
)
