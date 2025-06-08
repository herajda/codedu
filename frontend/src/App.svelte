<script lang="ts">
  import Router from 'svelte-spa-router'
  import wrap from 'svelte-spa-router/wrap'
  import type { RouteDefinition } from 'svelte-spa-router'
  import { asClassComponent } from 'svelte/legacy'
  import { get } from 'svelte/store'
  import { auth } from './lib/auth'

  import Login      from './routes/Login.svelte'
  import Register   from './routes/Register.svelte'
  import Dashboard  from './routes/Dashboard.svelte'
  import Admin      from './routes/Admin.svelte'
  import TeacherCls from './routes/TeacherClasses.svelte'
  import ClassPage  from './routes/ClassDetail.svelte'
  import MyClasses  from './routes/MyClasses.svelte'

  const isAuth   = () => !!get(auth)?.token
  const hasRole  = (role: string) => () => get(auth)?.role === role

  const routes: RouteDefinition = {
    '/':          asClassComponent(Login),
    '/login':     asClassComponent(Login),
    '/register':  asClassComponent(Register),

    '/dashboard': wrap({ component: asClassComponent(Dashboard), conditions:[isAuth], userData:{redirect:'/login'} }),

    // role-specific
    '/admin':     wrap({ component: asClassComponent(Admin),        conditions:[isAuth, hasRole('admin')],   userData:{redirect:'/login'} }),
    '/classes':   wrap({ component: asClassComponent(TeacherCls),   conditions:[isAuth, hasRole('teacher')], userData:{redirect:'/login'} }),
    '/my-classes':wrap({ component: asClassComponent(MyClasses),    conditions:[isAuth],                     userData:{redirect:'/login'} }),
    '/classes/:id': wrap({ component: asClassComponent(ClassPage),  conditions:[isAuth],                     userData:{redirect:'/login'} }),

    '*': asClassComponent(Login)
  }
</script>

<main>
  <Router {routes}/>
</main>
