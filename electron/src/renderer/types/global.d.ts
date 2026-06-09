import { DispatchAPI } from '../preload/index';

declare global {
  interface Window {
    dispatch: DispatchAPI;
  }
}
