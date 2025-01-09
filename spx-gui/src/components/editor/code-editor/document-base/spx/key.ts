import { categories } from '../../common'
import { defineConst } from './common'

export const keys = (
  [
    ['Key0', { en: 'Key 0', zh: '按键 0' }],
    ['Key1', { en: 'Key 1', zh: '按键 1' }],
    ['Key2', { en: 'Key 2', zh: '按键 2' }],
    ['Key3', { en: 'Key 3', zh: '按键 3' }],
    ['Key4', { en: 'Key 4', zh: '按键 4' }],
    ['Key5', { en: 'Key 5', zh: '按键 5' }],
    ['Key6', { en: 'Key 6', zh: '按键 6' }],
    ['Key7', { en: 'Key 7', zh: '按键 7' }],
    ['Key8', { en: 'Key 8', zh: '按键 8' }],
    ['Key9', { en: 'Key 9', zh: '按键 9' }],
    ['KeyA', { en: 'Key A', zh: '按键 A' }],
    ['KeyB', { en: 'Key B', zh: '按键 B' }],
    ['KeyC', { en: 'Key C', zh: '按键 C' }],
    ['KeyD', { en: 'Key D', zh: '按键 D' }],
    ['KeyE', { en: 'Key E', zh: '按键 E' }],
    ['KeyF', { en: 'Key F', zh: '按键 F' }],
    ['KeyG', { en: 'Key G', zh: '按键 G' }],
    ['KeyH', { en: 'Key H', zh: '按键 H' }],
    ['KeyI', { en: 'Key I', zh: '按键 I' }],
    ['KeyJ', { en: 'Key J', zh: '按键 J' }],
    ['KeyK', { en: 'Key K', zh: '按键 K' }],
    ['KeyL', { en: 'Key L', zh: '按键 L' }],
    ['KeyM', { en: 'Key M', zh: '按键 M' }],
    ['KeyN', { en: 'Key N', zh: '按键 N' }],
    ['KeyO', { en: 'Key O', zh: '按键 O' }],
    ['KeyP', { en: 'Key P', zh: '按键 P' }],
    ['KeyQ', { en: 'Key Q', zh: '按键 Q' }],
    ['KeyR', { en: 'Key R', zh: '按键 R' }],
    ['KeyS', { en: 'Key S', zh: '按键 S' }],
    ['KeyT', { en: 'Key T', zh: '按键 T' }],
    ['KeyU', { en: 'Key U', zh: '按键 U' }],
    ['KeyV', { en: 'Key V', zh: '按键 V' }],
    ['KeyW', { en: 'Key W', zh: '按键 W' }],
    ['KeyX', { en: 'Key X', zh: '按键 X' }],
    ['KeyY', { en: 'Key Y', zh: '按键 Y' }],
    ['KeyZ', { en: 'Key Z', zh: '按键 Z' }],
    ['KeyApostrophe', { en: 'Key Apostrophe', zh: '按键 Apostrophe' }],
    ['KeyBackslash', { en: 'Key Backslash', zh: '按键 Backslash' }],
    ['KeyBackspace', { en: 'Key Backspace', zh: '按键 Backspace' }],
    ['KeyCapsLock', { en: 'Key Caps Lock', zh: '按键 Caps Lock' }],
    ['KeyComma', { en: 'Key Comma', zh: '按键 Comma' }],
    ['KeyDelete', { en: 'Key Delete', zh: '按键 Delete' }],
    ['KeyDown', { en: 'Key Down', zh: '按键 Down' }],
    ['KeyEnd', { en: 'Key End', zh: '按键 End' }],
    ['KeyEnter', { en: 'Key Enter', zh: '按键 Enter' }],
    ['KeyEqual', { en: 'Key Equal', zh: '按键 Equal' }],
    ['KeyEscape', { en: 'Key Escape', zh: '按键 Escape' }],
    ['KeyF1', { en: 'Key F1', zh: '按键 F1' }],
    ['KeyF2', { en: 'Key F2', zh: '按键 F2' }],
    ['KeyF3', { en: 'Key F3', zh: '按键 F3' }],
    ['KeyF4', { en: 'Key F4', zh: '按键 F4' }],
    ['KeyF5', { en: 'Key F5', zh: '按键 F5' }],
    ['KeyF6', { en: 'Key F6', zh: '按键 F6' }],
    ['KeyF7', { en: 'Key F7', zh: '按键 F7' }],
    ['KeyF8', { en: 'Key F8', zh: '按键 F8' }],
    ['KeyF9', { en: 'Key F9', zh: '按键 F9' }],
    ['KeyF10', { en: 'Key F10', zh: '按键 F10' }],
    ['KeyF11', { en: 'Key F11', zh: '按键 F11' }],
    ['KeyF12', { en: 'Key F12', zh: '按键 F12' }],
    ['KeyGraveAccent', { en: 'Key Grave Accent', zh: '按键 Grave Accent' }],
    ['KeyHome', { en: 'Key Home', zh: '按键 Home' }],
    ['KeyInsert', { en: 'Key Insert', zh: '按键 Insert' }],
    ['KeyKP0', { en: 'Keypad 0', zh: '按键 0' }],
    ['KeyKP1', { en: 'Keypad 1', zh: '按键 1' }],
    ['KeyKP2', { en: 'Keypad 2', zh: '按键 2' }],
    ['KeyKP3', { en: 'Keypad 3', zh: '按键 3' }],
    ['KeyKP4', { en: 'Keypad 4', zh: '按键 4' }],
    ['KeyKP5', { en: 'Keypad 5', zh: '按键 5' }],
    ['KeyKP6', { en: 'Keypad 6', zh: '按键 6' }],
    ['KeyKP7', { en: 'Keypad 7', zh: '按键 7' }],
    ['KeyKP8', { en: 'Keypad 8', zh: '按键 8' }],
    ['KeyKP9', { en: 'Keypad 9', zh: '按键 9' }],
    ['KeyKPDecimal', { en: 'Keypad Decimal', zh: '按键 Decimal' }],
    ['KeyKPDivide', { en: 'Keypad Divide', zh: '按键 Divide' }],
    ['KeyKPEnter', { en: 'Keypad Enter', zh: '按键 Enter' }],
    ['KeyKPEqual', { en: 'Keypad Equal', zh: '按键 Equal' }],
    ['KeyKPMultiply', { en: 'Keypad Multiply', zh: '按键 Multiply' }],
    ['KeyKPSubtract', { en: 'Keypad Subtract', zh: '按键 Subtract' }],
    ['KeyLeft', { en: 'Key Left', zh: '按键 Left' }],
    ['KeyLeftBracket', { en: 'Key Left Bracket', zh: '按键 Left Bracket' }],
    ['KeyMenu', { en: 'Key Menu', zh: '按键 Menu' }],
    ['KeyMinus', { en: 'Key Minus', zh: '按键 Minus' }],
    ['KeyNumLock', { en: 'Key Num Lock', zh: '按键 Num Lock' }],
    ['KeyPageDown', { en: 'Key Page Down', zh: '按键 Page Down' }],
    ['KeyPageUp', { en: 'Key Page Up', zh: '按键 Page Up' }],
    ['KeyPause', { en: 'Key Pause', zh: '按键 Pause' }],
    ['KeyPeriod', { en: 'Key Period', zh: '按键 Period' }],
    ['KeyPrintScreen', { en: 'Key Print Screen', zh: '按键 Print Screen' }],
    ['KeyRight', { en: 'Key Right', zh: '按键 Right' }],
    ['KeyRightBracket', { en: 'Key Right Bracket', zh: '按键 Right Bracket' }],
    ['KeyScrollLock', { en: 'Key Scroll Lock', zh: '按键 Scroll Lock' }],
    ['KeySemicolon', { en: 'Key Semicolon', zh: '按键 Semicolon' }],
    ['KeySlash', { en: 'Key Slash', zh: '按键 Slash' }],
    ['KeySpace', { en: 'Key Space', zh: '按键 Space' }],
    ['KeyTab', { en: 'Key Tab', zh: '按键 Tab' }],
    ['KeyUp', { en: 'Key Up', zh: '按键 Up' }],
    ['KeyAlt', { en: 'Key Alt', zh: '按键 Alt' }],
    ['KeyControl', { en: 'Key Control', zh: '按键 Control' }],
    ['KeyShift', { en: 'Key Shift', zh: '按键 Shift' }],
    ['KeyMax', { en: 'Key Max', zh: '按键 Max' }],
    ['KeyAny', { en: 'Any key', zh: '任意按键' }]
  ] as const
).map(([key, desc]) => defineConst(key, [categories.sensing.keyboard], desc))