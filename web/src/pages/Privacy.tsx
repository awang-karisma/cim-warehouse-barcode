import { type Component } from 'solid-js';
import { A } from '@solidjs/router';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '~/components/ui/card';

export const Privacy: Component = () => {
  return (
    <div class="min-h-screen w-full flex items-center justify-center p-4 py-8 bg-muted/30">
      <Card class="w-full max-w-2xl shadow-lg border-border my-8">
        <CardHeader class="space-y-1 pb-4">
          <CardTitle class="text-2xl font-bold tracking-tight">Privacy Policy</CardTitle>
          <CardDescription>Last Updated: [YYYY-MM-DD]</CardDescription>
        </CardHeader>
        <CardContent class="space-y-6 text-sm text-muted-foreground">
          <section class="space-y-2">
            <p>
              [Operator Name] (&ldquo;we&rdquo;, &ldquo;us&rdquo;, &ldquo;our&rdquo;) operates the Warehouse Console application. This Privacy Policy explains how we collect, use, and protect your information when you use our service.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">1. Information We Collect</h2>
            <p>
              When you sign in to Warehouse Console using Google OAuth, we collect your name, email address, and profile picture provided by your Google account. This information is stored in our application database to identify you, manage your active session, and apply role-based access control.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">2. How We Use Your Information</h2>
            <p>
              We use your information exclusively for the following purposes:
            </p>
            <ul class="list-disc pl-5 space-y-1">
              <li>Authenticating your identity and establishing your session.</li>
              <li>Enforcing role-based access control (guest, user, admin, superadmin) across warehouse data.</li>
              <li>Displaying your name and profile picture within the application user interface.</li>
            </ul>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">3. Data Storage and Retention</h2>
            <p>
              Your account details are stored in a server-side SQLite database. We retain this data for as long as your account remains active in the system. We do not sell, rent, or share your personal information with any third parties, and your data is never used for advertising or marketing.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">4. Third-Party Services</h2>
            <p>
              Warehouse Console utilizes Google Identity Services for single sign-on authentication. Your authentication interaction is governed by Google&rsquo;s Privacy Policy and terms of service.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">5. Data Deletion</h2>
            <p>
              You have the right to request deletion of your account and associated personal data. To initiate a data deletion request, contact the application administrator at <span class="font-mono text-foreground">[Contact Email]</span>. Upon receipt, an administrator will remove your user record from the database.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">6. Changes to This Policy</h2>
            <p>
              We may revise this Privacy Policy periodically. Any updates will be posted on this page with a revised &ldquo;Last Updated&rdquo; date. Continued use of the application after changes constitute acceptance of the updated policy.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">7. Contact</h2>
            <p>
              If you have any questions or concerns regarding this Privacy Policy or our data practices, please reach out to <span class="font-mono text-foreground">[Contact Email]</span>.
            </p>
          </section>

          <div class="pt-4 border-t border-border flex items-center justify-between">
            <A href="/" class="text-sm text-primary hover:underline font-medium">
              &larr; Back to sign in
            </A>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};
