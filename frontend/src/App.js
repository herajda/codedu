import React, { useState } from 'react';

function Registration() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState('');

  const handleRegister = async (event) => {
    event.preventDefault();

    // Simple validation
    if (!username || !password) {
      setMessage('Username and password are required');
      return;
    }

    try {
      const response = await fetch('http://localhost:18080/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ username, password }),
      });

      const data = await response.text();
      setMessage(data);
    } catch (error) {
      console.error('Error during registration:', error);
      setMessage('Failed to register');
    }
  };

  return (
    <div>
      <h2>Register</h2>
      <form onSubmit={handleRegister}>
        <input type="text" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="Username" required />
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Password" required />
        <button type="submit">Register</button>
      </form>
      {message && <p>{message}</p>}
    </div>
  );
}

export default Registration;
