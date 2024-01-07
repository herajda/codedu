import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

function HomePage() {
  const [classes, setClasses] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    const token = localStorage.getItem('authToken'); // Assuming the token is stored in localStorage
    if (!token) {
      navigate('/login');
      return;
    }

    const fetchClasses = async () => {
      try {
        const response = await fetch('https://api.example.com/my-classes', {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        });

        if (!response.ok) {
          throw new Error('Failed to fetch classes');
        }

        const data = await response.json();
        setClasses(data.classes);
      } catch (error) {
        console.error(error.message);
        // Handle errors (like redirect to login if token is invalid)
      } finally {
        setIsLoading(false);
      }
    };

    fetchClasses();
  }, [navigate]);

  if (isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <div>
      <h1>Home Page</h1>
      <h2>My Classes</h2>
      <table>
        <thead>
          <tr>
            <th>Class Name</th>
            <th>Subject</th>
            <th>Teacher</th>
            {/* Add more columns as needed */}
          </tr>
        </thead>
        <tbody>
          {classes.map((classItem, index) => (
            <tr key={index}>
              <td>{classItem.name}</td>
              <td>{classItem.subject}</td>
              <td>{classItem.teacher}</td>
              {/* Render other class details here */}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default HomePage;
