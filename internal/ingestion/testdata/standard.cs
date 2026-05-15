using System;
using System.Collections.Generic;

namespace Azimuth.Services
{
    public class UserService : BaseService, IUserService
    {
        public string ServiceName { get; set; }
        public int RetryCount { get; }

        public User GetUser(int id)
        {
            return null;
        }

        public override void Validate(string input)
        {
            if (string.IsNullOrEmpty(input))
                throw new ArgumentException("Input cannot be empty");
        }

        private bool IsValid(string value)
        {
            return !string.IsNullOrEmpty(value);
        }
    }
}
