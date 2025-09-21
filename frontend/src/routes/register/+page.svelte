<script lang="ts">
    import { goto } from '$app/navigation'
    import { sha256 } from '$lib/hash'

    let firstName = ''
    let lastName = ''
    let email = ''
    let password = ''
    let passwordConfirm = ''
    let error = ''

    $: hasMinLength = password.length > 8
    $: hasLetter = /[A-Za-z]/.test(password)
    $: hasNumber = /\d/.test(password)
    $: meetsPasswordRules = hasMinLength && hasLetter && hasNumber
    $: passwordsMatch = passwordConfirm.length === 0 ? false : password === passwordConfirm
    $: canSubmit = firstName.trim().length > 0 && lastName.trim().length > 0 && email.trim().length > 0 && meetsPasswordRules && passwordsMatch
  
    async function submit() {
      error = ''
      if (!firstName.trim() || !lastName.trim()) {
        error = 'Please provide your first and last name.'
        return
      }
      if (!meetsPasswordRules) {
        error = 'Password must be longer than 8 characters and include letters and numbers.'
        return
      }
      if (!passwordsMatch) {
        error = 'Passwords must match.'
        return
      }

      const res = await fetch('/api/register', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          firstName: firstName.trim(),
          lastName: lastName.trim(),
          email: email.trim(),
          password: await sha256(password)
        })
      })
      if (res.status === 201) {
        goto('/login')
      } else {
        error = (await res.json()).error
      }
    }
  </script>
  
  <h1 class="text-3xl font-bold text-center mb-6">Register</h1>
  <div class="flex justify-center">
    <form on:submit|preventDefault={submit} class="card w-full max-w-sm bg-base-100 shadow p-6 space-y-4">
      <input type="text" bind:value={firstName} placeholder="First name" required class="input input-bordered w-full" />
      <input type="text" bind:value={lastName} placeholder="Last name" required class="input input-bordered w-full" />
      <input type="email" bind:value={email} placeholder="Email" required class="input input-bordered w-full" />
      <div class="space-y-2">
        <input type="password" bind:value={password} placeholder="Password" required class="input input-bordered w-full" />
        <div class="bg-base-200 rounded-lg p-3 text-sm space-y-2">
          <p class="font-semibold text-base-content">Password requirements</p>
          <ul class="space-y-1">
            <li class={`flex items-center gap-2 ${hasMinLength ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasMinLength ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>At least 9 characters</span>
            </li>
            <li class={`flex items-center gap-2 ${hasLetter ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasLetter ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>Includes a letter</span>
            </li>
            <li class={`flex items-center gap-2 ${hasNumber ? 'text-success' : 'text-base-content/70'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${hasNumber ? 'bg-success' : 'bg-base-300'}`}></span>
              <span>Includes a number</span>
            </li>
            <li class={`flex items-center gap-2 ${passwordConfirm.length === 0 ? 'text-base-content/70' : passwordsMatch ? 'text-success' : 'text-error'}`}>
              <span class={`inline-flex w-2 h-2 rounded-full ${passwordConfirm.length === 0 ? 'bg-base-300' : passwordsMatch ? 'bg-success' : 'bg-error'}`}></span>
              <span>Passwords match</span>
            </li>
          </ul>
        </div>
      </div>
      <input type="password" bind:value={passwordConfirm} placeholder="Confirm password" required class="input input-bordered w-full" />
      <button type="submit" class="btn btn-primary w-full" disabled={!canSubmit}>Register</button>
    </form>
  </div>
  {#if error}
    <p class="text-error text-center mt-4">{error}</p>
  {/if}
  <p class="text-center mt-4">
    Already have an account? <a href="/login" class="link link-primary">Log in</a>
  </p>
  
