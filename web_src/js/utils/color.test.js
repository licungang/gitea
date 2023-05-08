import {test, expect} from 'vitest';
import {getRGBColorFromHex, useLightTextOnBackground} from './color.js';

test('getRGBColorFromHex', () => {
  expect(getRGBColorFromHex('2b8685')).toEqual([43, 134, 133]);
  expect(getRGBColorFromHex('2b8786')).toEqual([43, 135, 134]);
  expect(getRGBColorFromHex('2c8786')).toEqual([44, 135, 134]);
  expect(getRGBColorFromHex('3bb6b3')).toEqual([59, 182, 179]);
  expect(getRGBColorFromHex('7c7268')).toEqual([124, 114, 104]);
  expect(getRGBColorFromHex('#7e716c')).toEqual([126, 113, 108]);
  expect(getRGBColorFromHex('#807070')).toEqual([128, 112, 112]);
  expect(getRGBColorFromHex('#81706d')).toEqual([129, 112, 109]);
  expect(getRGBColorFromHex('#d73a4a')).toEqual([215, 58, 74]);
  expect(getRGBColorFromHex('#0075ca')).toEqual([0, 117, 202]);
});

test('useLightTextOnBackground', () => {
  expect(useLightTextOnBackground(215, 58, 74)).toBe(true);
  expect(useLightTextOnBackground(0, 117, 202)).toBe(true);
  expect(useLightTextOnBackground(207, 211, 215)).toBe(false);
  expect(useLightTextOnBackground(162, 238, 239)).toBe(false);
  expect(useLightTextOnBackground(112, 87, 255)).toBe(true);
  expect(useLightTextOnBackground(0, 134, 114)).toBe(true);
  expect(useLightTextOnBackground(228, 230, 105)).toBe(false);
  expect(useLightTextOnBackground(216, 118, 227)).toBe(true);
  expect(useLightTextOnBackground(255, 255, 255)).toBe(false);
  expect(useLightTextOnBackground(43, 134, 133)).toBe(true);
  expect(useLightTextOnBackground(43, 135, 134)).toBe(true);
  expect(useLightTextOnBackground(44, 135, 134)).toBe(true);
  expect(useLightTextOnBackground(59, 182, 179)).toBe(true);
  expect(useLightTextOnBackground(124, 114, 104)).toBe(true);
  expect(useLightTextOnBackground(126, 113, 108)).toBe(true);
  expect(useLightTextOnBackground(129, 112, 109)).toBe(true);
  expect(useLightTextOnBackground(128, 112, 112)).toBe(true);
});
