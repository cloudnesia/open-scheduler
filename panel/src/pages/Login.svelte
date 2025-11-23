<script>
  import { Card, Button, Label, Input, Alert } from 'flowbite-svelte';
  import { auth } from '../api/client';
  import { authStore } from '../stores/authStore';
  import { navigate } from 'svelte-routing';

  let username = '';
  let password = '';
  let error = '';
  let loading = false;

  async function handleLogin() {
    error = '';
    loading = true;

    try {
      const response = await auth.login(username, password);
      const expiresAt = Date.now() + (response.expires_in * 1000);
      
      authStore.set({
        token: response.token,
        username: response.username,
        expiresAt,
      });

      navigate('/dashboard');
    } catch (err) {
      error = err.message || 'Login failed';
    } finally {
      loading = false;
    }
  }

  function handleSubmit(e) {
    e.preventDefault();
    handleLogin();
  }
</script>

<div class="flex items-center justify-center min-h-screen bg-gray-100 dark:bg-gray-900">
  <div class="w-full max-w-md p-4">
    <Card class="p-8">
      <div class="mb-6 text-center">
        <h1 class="text-3xl font-bold text-gray-900 dark:text-white mb-2">Open Scheduler</h1>
        <p class="text-gray-600 dark:text-gray-400">Control Panel</p>
      </div>

      {#if error}
        <Alert color="red" class="mb-4">
          <span class="font-medium">Error!</span> {error}
        </Alert>
      {/if}

      <form on:submit={handleSubmit} class="space-y-6">
        <div>
          <Label for="username" class="mb-2">Username</Label>
          <Input
            id="username"
            type="text"
            bind:value={username}
            required
            placeholder="Enter username"
          />
        </div>

        <div>
          <Label for="password" class="mb-2">Password</Label>
          <Input
            id="password"
            type="password"
            bind:value={password}
            required
            placeholder="Enter password"
          />
        </div>

        <Button type="submit" class="w-full" disabled={loading}>
          {loading ? 'Signing in...' : 'Sign in'}
        </Button>
      </form>

      <div class="mt-4 text-sm text-center text-gray-500 dark:text-gray-400">
        Default credentials: admin / admin123
      </div>
    </Card>
  </div>
</div>

