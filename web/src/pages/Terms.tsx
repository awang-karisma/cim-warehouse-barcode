import { type Component } from 'solid-js';
import { A } from '@solidjs/router';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '~/components/ui/card';

export const Terms: Component = () => {
  return (
    <div class="min-h-screen w-full flex items-center justify-center p-4 py-8 bg-muted/30">
      <Card class="w-full max-w-2xl shadow-lg border-border my-8">
        <CardHeader class="space-y-1 pb-4">
          <CardTitle class="text-2xl font-bold tracking-tight">Terms of Service</CardTitle>
          <CardDescription>Last Updated: [YYYY-MM-DD]</CardDescription>
        </CardHeader>
        <CardContent class="space-y-6 text-sm text-muted-foreground">
          <section class="space-y-2">
            <p>
              By accessing or using the Warehouse Console application operated by [Operator Name] (&ldquo;we&rdquo;, &ldquo;us&rdquo;, &ldquo;our&rdquo;), you agree to be bound by these Terms of Service. If you do not agree to these terms, please do not access or use the application.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">1. Description of Service</h2>
            <p>
              Warehouse Console is an internal order tracking, normalization, and visualization system. The service ingests order data from connected spreadsheets, normalizes customer identifiers and delivery timelines, and provides operational reporting dashboards and search tools for authorized personnel.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">2. Account and Access</h2>
            <p>
              Access to Warehouse Console requires authentication through Google OAuth. Accounts are provisioned with specific role levels (guest, user, admin, superadmin). Newly signed-in accounts default to guest status and require administrator approval before application data and dashboards can be accessed.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">3. Acceptable Use</h2>
            <p>
              You agree to use Warehouse Console solely for authorized internal business operations related to warehouse inventory and order management. You agree not to:
            </p>
            <ul class="list-disc pl-5 space-y-1">
              <li>Attempt to circumvent security controls or access data beyond your assigned role permissions.</li>
              <li>Interfere with, disrupt, or compromise the integrity or performance of the service.</li>
              <li>Share session credentials or authorize unauthorized third parties to access the service under your account.</li>
            </ul>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">4. Data Accuracy and Disclaimers</h2>
            <p>
              Order records are synchronized from external human-maintained spreadsheets and processed automatically. While the system applies automated normalization routines, [Operator Name] does not warrant or guarantee the complete accuracy, completeness, or timeliness of ingested source data. Users should independently verify critical operational details before taking high-impact inventory or shipment actions.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">5. Limitation of Liability</h2>
            <p>
              Warehouse Console is provided on an &ldquo;as is&rdquo; and &ldquo;as available&rdquo; basis without warranties of any kind, whether express or implied. To the maximum extent permitted by applicable law, [Operator Name] shall not be liable for any direct, indirect, incidental, special, consequential, or punitive damages arising from your use of or inability to use the service.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">6. Modifications to Terms</h2>
            <p>
              We reserve the right to revise or update these Terms of Service at any time. Updated terms will be posted directly to this page with a revised &ldquo;Last Updated&rdquo; date. Continued access or use of Warehouse Console following such modifications constitutes your acceptance of the revised terms.
            </p>
          </section>

          <section class="space-y-2">
            <h2 class="text-base font-semibold text-foreground">7. Contact</h2>
            <p>
              For inquiries or support regarding these Terms of Service, please contact the system administrator at <span class="font-mono text-foreground">[Contact Email]</span>.
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
