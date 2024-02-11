import {pngChunks, pngInfo} from './image.js';

const pngNoPhys = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAADUlEQVQIHQECAP3/AAAAAgABzePRKwAAAABJRU5ErkJggg==';
const pngPhys = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAACXBIWXMAABYlAAAWJQFJUiTwAAAAEElEQVQI12OQNZcAIgYIBQAL8gGxdzzM0A==';

async function toBlob(datauri) {
  return await (await globalThis.fetch(datauri)).blob();
}

test('pngChunks', async () => {
  expect(await pngChunks(await toBlob(pngNoPhys))).toEqual([
    {name: 'IHDR', data: new Uint8Array([0, 0, 0, 1, 0, 0, 0, 1, 8, 0, 0, 0, 0])},
    {name: 'IDAT', data: new Uint8Array([8, 29, 1, 2, 0, 253, 255, 0, 0, 0, 2, 0, 1])},
    {name: 'IEND', data: new Uint8Array([])},
  ]);
  expect(await pngChunks(await toBlob(pngPhys))).toEqual([
    {name: 'IHDR', data: new Uint8Array([0, 0, 0, 2, 0, 0, 0, 2, 8, 2, 0, 0, 0])},
    {name: 'pHYs', data: new Uint8Array([0, 0, 22, 37, 0, 0, 22, 37, 1])},
    {name: 'IDAT', data: new Uint8Array([8, 215, 99, 144, 53, 151, 0, 34, 6, 8, 5, 0, 11, 242, 1, 177])},
  ]);
});

test('pngInfo', async () => {
  expect(await pngInfo(await toBlob(pngNoPhys))).toEqual({width: 1, dppx: 1});
  expect(await pngInfo(await toBlob(pngPhys))).toEqual({width: 2, dppx: 2});
});
