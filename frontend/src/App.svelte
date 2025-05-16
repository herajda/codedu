<script lang="ts">
  import Router from 'svelte-spa-router'
  import wrap from 'svelte-spa-router/wrap'
  import { get } from 'svelte/store'
  import Login from './routes/Login.svelte'
  import Register from './routes/Register.svelte'
  import Dashboard from './routes/Dashboard.svelte'
  import { auth } from './lib/auth'

  const isAuth = () => !!get(auth)?.token

  const Protected = wrap({
    component: Dashboard as any,
    conditions: [isAuth],
    userData: { redirect: '/login' }
  })

  const routes = {
    '/': Login,
    '/login': Login,
    '/register': Register,
    '/dashboard': Protected,
    '*': Login
  }
</script>

<main>
  <Router {routes} />
</main>
