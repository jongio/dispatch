import { Tray, Menu, app, nativeImage, BrowserWindow } from 'electron';
import { join } from 'path';

export class DispatchTray {
  private tray: Tray | null = null;
  private mainWindow: BrowserWindow | null = null;
  private attentionCount = 0;

  create(mainWindow: BrowserWindow): void {
    this.mainWindow = mainWindow;
    const iconPath = join(__dirname, '../../build/icon.png');
    const icon = nativeImage.createFromPath(iconPath).resize({ width: 16, height: 16 });
    this.tray = new Tray(icon);
    this.tray.setToolTip('Dispatch');
    this.updateMenu();

    this.tray.on('click', () => this.toggleWindow());
  }

  updateAttentionCount(count: number): void {
    this.attentionCount = count;
    this.updateMenu();
    const tooltip = count > 0 ? `Dispatch (${count} waiting)` : 'Dispatch';
    this.tray?.setToolTip(tooltip);
  }

  private toggleWindow(): void {
    if (!this.mainWindow) return;
    if (this.mainWindow.isVisible() && this.mainWindow.isFocused()) {
      this.mainWindow.hide();
    } else {
      this.mainWindow.show();
      this.mainWindow.focus();
    }
  }

  private updateMenu(): void {
    const contextMenu = Menu.buildFromTemplate([
      { label: `Dispatch${this.attentionCount > 0 ? ` (${this.attentionCount})` : ''}`, enabled: false },
      { type: 'separator' },
      { label: 'Show', click: () => { this.mainWindow?.show(); this.mainWindow?.focus(); } },
      { label: 'Hide', click: () => { this.mainWindow?.hide(); } },
      { type: 'separator' },
      { label: 'Quit', click: () => app.quit() },
    ]);
    this.tray?.setContextMenu(contextMenu);
  }

  destroy(): void {
    this.tray?.destroy();
    this.tray = null;
  }
}
