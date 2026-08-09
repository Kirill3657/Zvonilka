// src/App.js
import React, { useState } from 'react';
import AuthByEmail from './components/AuthByEmail';
import Chat from './components/Chat';

function App() {
  const [user, setUser] = useState(null); // { email, userId }

  if (!user) {
    return <AuthByEmail onAuthSuccess={(email, userId) => setUser({ email, userId })} />;
  }

  return <Chat userId={user.userId} userEmail={user.email} />;
}

export default App;