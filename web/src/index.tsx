import { ROUTES } from './routes';

import { ColorModeProvider, ColorModeScript, createLocalStorageManager } from '@kobalte/core';
import { Navigate, Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import './app.css';

import { App } from './App';
import { AuthProvider } from './auth';
import { I18nProvider } from './i18n';
import { Barcode } from './pages/Barcode';
import { Privacy } from './pages/Privacy';
import { Terms } from './pages/Terms';
import { UserManagement } from './pages/UserManagement';
import { RoleManagement } from './pages/RoleManagement';
import { RBACTemplate } from './pages/RBACTemplate';

const storageManager = createLocalStorageManager('warehouse-barcode-ui-theme');

const root = document.getElementById('root');
if (root) {
  render(
    () => (
      <>
        <ColorModeScript storageType={storageManager.type} storageKey="warehouse-barcode-ui-theme" />
        <ColorModeProvider storageManager={storageManager}>
          <AuthProvider>
            <I18nProvider>
              <Router root={App}>
                <Route path={ROUTES.root} component={() => <Navigate href={ROUTES.barcode} />} />
                <Route path={ROUTES.barcode} component={Barcode} />
                <Route path={ROUTES.users} component={UserManagement} />
                <Route path={ROUTES.roles} component={RoleManagement} />
                <Route path={ROUTES.rbacTemplate} component={RBACTemplate} />
                <Route path={ROUTES.privacy} component={Privacy} />
                <Route path={ROUTES.terms} component={Terms} />
              </Router>
            </I18nProvider>
          </AuthProvider>
        </ColorModeProvider>
      </>
    ),
    root
  );
}
