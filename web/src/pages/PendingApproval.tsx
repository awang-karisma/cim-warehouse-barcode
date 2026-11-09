import { type Component } from 'solid-js';
import { useI18n } from '~/i18n';
import { useAuth } from '~/auth';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '~/components/ui/card';
import { Button } from '~/components/ui/button';
import { Avatar, AvatarImage, AvatarFallback } from '~/components/ui/avatar';
import { IconClock, IconLogout } from '@tabler/icons-solidjs';

export const PendingApproval: Component = () => {
  const { t } = useI18n();
  const auth = useAuth();
  const currentUser = () => auth.user();

  const userInitial = () => {
    const name = currentUser()?.name || currentUser()?.email || 'U';
    return name.charAt(0).toUpperCase();
  };

  return (
    <div class="min-h-screen w-full flex items-center justify-center p-4 bg-muted/30">
      <Card class="w-full max-w-md shadow-lg border-border text-center">
        <CardHeader class="flex flex-col items-center space-y-3 pb-4">
          <div class="relative">
            <Avatar class="h-20 w-20 border-2 border-primary/20 shadow-sm">
              <AvatarImage src={currentUser()?.picture} alt={currentUser()?.name} />
              <AvatarFallback class="text-xl font-bold bg-primary/10 text-primary">
                {userInitial()}
              </AvatarFallback>
            </Avatar>
            <div class="absolute -bottom-1 -right-1 rounded-full bg-amber-500 text-white p-1 shadow">
              <IconClock class="h-4 w-4" />
            </div>
          </div>
          <div>
            <CardTitle class="text-xl font-bold">{currentUser()?.name || t('pendingApproval.newUser')}</CardTitle>
            <CardDescription class="text-sm font-mono mt-0.5">{currentUser()?.email}</CardDescription>
          </div>
        </CardHeader>

        <CardContent class="space-y-6 pt-2 pb-6">
          <div class="rounded-lg bg-amber-500/10 border border-amber-500/20 p-4 text-sm text-foreground">
            <p class="font-medium text-amber-600 dark:text-amber-400 mb-1">{t('pendingApproval.title')}</p>
            <p class="text-muted-foreground text-xs leading-relaxed">
              {t('pendingApproval.description')}
            </p>
          </div>

          <div class="flex flex-col gap-2">
            <Button
              variant="outline"
              class="w-full gap-2 border-border hover:bg-muted"
              onClick={() => auth.refetch()}
            >
              {t('pendingApproval.checkStatus')}
            </Button>
            <Button
              variant="ghost"
              class="w-full gap-2 text-muted-foreground hover:text-foreground"
              onClick={() => auth.logout()}
            >
              <IconLogout class="h-4 w-4" />
              {t('common.signOut')}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};
