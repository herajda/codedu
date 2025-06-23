<script lang="ts">
import Router from 'svelte-spa-router'
import Layout from './lib/Layout.svelte'
  import wrap from 'svelte-spa-router/wrap'
  import type { RouteDefinition } from 'svelte-spa-router'
  import { get } from 'svelte/store'
  import { auth } from './lib/auth'

  import Login      from './routes/Login.svelte'
  import Register   from './routes/Register.svelte'
  import Dashboard  from './routes/Dashboard.svelte'
  import Admin      from './routes/Admin.svelte'
  import TeacherCls from './routes/TeacherClasses.svelte'
  import ClassPage  from './routes/ClassDetail.svelte'
  import MyClasses  from './routes/MyClasses.svelte'
  import AssignmentPage from './routes/AssignmentDetail.svelte'
  import SubmissionPage from './routes/SubmissionDetail.svelte'

  const isAuth   = () => !!get(auth)?.token
  const hasRole  = (role: string) => () => get(auth)?.role === role

  const routes: RouteDefinition = {
    '/':          Login,
    '/login':     Login,
    '/register':  Register,

    '/dashboard': wrap({ component: Dashboard, conditions:[isAuth], userData:{redirect:'/login'} }),

    // role-specific
    '/admin':     wrap({ component: Admin,        conditions:[isAuth, hasRole('admin')],   userData:{redirect:'/login'} }),
    '/classes':   wrap({ component: TeacherCls,   conditions:[isAuth, hasRole('teacher')], userData:{redirect:'/login'} }),
    '/my-classes':wrap({ component: MyClasses,    conditions:[isAuth],                     userData:{redirect:'/login'} }),
    '/classes/:id': wrap({ component: ClassPage,  conditions:[isAuth],                     userData:{redirect:'/login'} }),
    '/assignments/:id': wrap({ component: AssignmentPage, conditions:[isAuth], userData:{redirect:'/login'} }),
    '/submissions/:id': wrap({ component: SubmissionPage, conditions:[isAuth], userData:{redirect:'/login'} }),

    '*': Login
  }
</script>

<Layout>
  <Router {routes} />
</Layout>
