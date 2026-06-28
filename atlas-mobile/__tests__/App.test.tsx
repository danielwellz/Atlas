/**
 * @format
 */

import React from 'react';
import ReactTestRenderer from 'react-test-renderer';
import App from '../App';

test('renders authentication entry screen by default', async () => {
  let renderer: ReactTestRenderer.ReactTestRenderer;

  await ReactTestRenderer.act(async () => {
    renderer = ReactTestRenderer.create(<App />);
  });

  expect(renderer!.toJSON()).toBeTruthy();

  await ReactTestRenderer.act(async () => {
    renderer!.unmount();
  });
});
