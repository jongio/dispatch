import type { DispatchAPI } from '../../preload/index';

export type { Config } from '../../preload/index';

declare global {
  interface Window {
    dispatch: DispatchAPI;
  }
}
