# code-grader: specification of the idea
 
I want to create an easy-to-self-host software "code-grader" designed to automatically evaluate and grade coding assignments or projects. It aims to streamline the process of code assessment. A major inspiration is MFF's [ReCodEx](https://github.com/ReCodEx). 

To simplify, the code-grader will grade only Python scripts. This functionality could be easily extended in the future.  

### Overview of the program

The first thing you see, when you open the web app is a login page. There you'll have a chance to log in as a student or a teacher (possibly admin). Or create an account, but only for students, the teachers have to be added by admin. 

#### Create an account 
- only for students 
- form with
  - first name
  - last name
  - nickname
  - (no email, as that would require an email server, which we don't have).

#### When you log in as a teacher: 
You have several options:

- manage classes and add students to them
- add assignment to a class and specify it - name of the assignment, what the student should do
  - for each assignment the teacher can
    - add tests; each test have these parameters
      - input (stdin)
      - required output (stdout)
      - input visibility to student - default FALSE
      - (if I have time: max time, memory)
    - say how the assignments should be graded - success/fail - percentage of tests correct/all tests have to be correct 
    - specify the deadline (for simplicity only one)


#### When you log in as a student

You have several options to do:

- see in which classes you are added
- in each class the student can see their assignments, deadlines and tests
- the student will be allowed to upload their solution for each assignment and see how the tests go 


### Technical specification

The code-grader will have a frontend developed using HTML/CSS + React and a backend in C++ with a PostgreSQL database.

I'll use [Crow](https://crowcpp.org/master/) and [Boost.Asio](https://www.boost.org/doc/libs/1_78_0/doc/html/boost_asio.html) for communicating between the backend server and the frontend website. I am also considering using [Oat++](https://oatpp.io/). Crow can also handle uploading files. 

A big technical question is how the tests will be run. To ensure safety, all tests will involve creating a Docker container just for the purpose of running one test. 
This could be done using simple system calls.

I might need to implement a Task Queue for running the tests. I don't know which exact approach I'll use, but I have these options:
- using standard library thread support: `std::thread`, `std::mutex` , `std::queue` , `std::deque`
- using `std::async` and `std::future`
- third-party libraries
  - [Boost.Thread](https://www.boost.org/doc/libs/1_78_0/doc/html/thread.html)
  - [Intel Threading Building Blocks](https://www.intel.com/content/www/us/en/developer/tools/oneapi/onetbb.html)
  - [Poco Libraries](https://pocoproject.org/)

For secure password hashing I'll use Argon2 hashing function. 

All the data will be stored in a SQL database. I'll use PostgreSQL and [libpqxx](https://pqxx.org/libpqxx/). 

Finally, as mentioned, I want the whole software to be easily self-hosted. So the goal is to produce easy-to-deploy Docker images. 
