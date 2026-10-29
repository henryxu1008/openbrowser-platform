import { afterEach, expect, it, vi } from 'vitest';
import { fetchSettings } from '../src/modules/settings/api';
import { projectConfig } from '../src/config/projectBase.config';

afterEach(() => vi.unstubAllGlobals());

it.each(['', '  ', 'Ant Browser', ' Ant Browser ', 'Ant Chrome'])(
  'upgrades the stored default app name %j while retaining other settings',
  async (appName) => {
    const getItem = vi.fn(() => JSON.stringify({ appName, theme: 'dark', gcPercent: 80 }));
    vi.stubGlobal('localStorage', { getItem });

    const settings = await fetchSettings();

    expect(getItem).toHaveBeenCalledWith('app_settings');
    expect(settings).toMatchObject({ appName: projectConfig.name, theme: 'dark', gcPercent: 80 });
  },
);

it('preserves a custom stored app name', async () => {
  vi.stubGlobal('localStorage', { getItem: () => JSON.stringify({ appName: '团队专用浏览器' }) });
  expect((await fetchSettings()).appName).toBe('团队专用浏览器');
});
