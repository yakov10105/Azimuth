using System;
using Microsoft.AspNetCore.Mvc;

namespace Azimuth.Api
{
    [ApiController]
    [Route("api/[controller]")]
    public class UsersController
    {
        public string ControllerId { get; set; }

        [HttpGet("{id}")]
        [Authorize(Roles = "Admin")]
        public User GetUser(int id)
        {
            return null;
        }

        [HttpPost]
        public void CreateUser(User user)
        {
        }

        [HttpDelete("{id}")]
        public void DeleteUser(int id)
        {
        }
    }
}
