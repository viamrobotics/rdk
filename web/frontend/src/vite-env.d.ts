/* eslint-disable multiline-comment-style */
/* eslint-disable spaced-comment */
/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module '*.txt' {
  const value: string;
  export default value;
}
