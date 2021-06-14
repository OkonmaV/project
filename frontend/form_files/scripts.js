$(document).ready(function () {

  $('.text-show__link').click(function () {
    if ($('.text-show__link, .text-show p').hasClass('active')) {
      $('.text-show__link, .text-show p').removeClass('active');
    } else {
      $('.text-show__link, .text-show p').addClass('active');
    }
  });

  $('.filter-type-link').click(function () {
    if ($('.filter-type-link,.filter-type').hasClass('open')) {
      $('.filter-type-link,.filter-type').removeClass('open');
    } else {
      $('.filter-type-link,.filter-type').addClass('open');
    }
  });

  $('.bookmark').on('click', function (event) {
    event.preventDefault();
    $(this).toggleClass('active');
  });

  $('.photo-slider .owl-carousel').owlCarousel({
    items: 1,
    loop: true,
    margin: 6,
    nav: true,
    dots: true,
    autoHeight: true,
    responsive: {
      0: {
        nav: false,
        dots: true,
        stagePadding: 16,
      },
      768: {
        nav: true,
        dots: true,
      }
    }
  });


  $(".custom-select").each(function () {
    var classes = $(this).attr("class"),
      id = $(this).attr("id"),
      name = $(this).attr("name");
    var template = '<div class="' + classes + '">';
    template +=
      '<span class="custom-select-trigger">' +
      $(this).attr("placeholder") +
      "</span>";
    template += '<div class="custom-options">';
    $(this)
      .find("option")
      .each(function () {
        template +=
          '<span class="custom-option ' +
          $(this).attr("class") +
          '" data-value="' +
          $(this).attr("value") +
          '">' +
          $(this).html() +
          "</span>";
      });
    template += "</div></div>";

    $(this).wrap('<div class="custom-select-wrapper"></div>');
    $(this).hide();
    $(this).after(template);
  });
  $(".custom-option:first-of-type").hover(
    function () {
      $(this)
        .parents(".custom-options")
        .addClass("option-hover");
    },
    function () {
      $(this)
        .parents(".custom-options")
        .removeClass("option-hover");
    }
  );
  $(".custom-select-trigger").on("click", function () {
    $("html").one("click", function () {
      $(".custom-select").removeClass("opened");
    });
    $(this)
      .parents(".custom-select")
      .toggleClass("opened");
    event.stopPropagation();
  });
  $(".custom-option").on("click", function () {
    $(this)
      .parents(".custom-select-wrapper")
      .find("select")
      .val($(this).data("value"));
    $(this)
      .parents(".custom-options")
      .find(".custom-option")
      .removeClass("selection");
    $(this).addClass("selection");
    $(this)
      .parents(".custom-select")
      .removeClass("opened");
    $(this)
      .parents(".custom-select")
      .find(".custom-select-trigger")
      .text($(this).text());
  });




  $(document).ready(function () {
    var $inpt = $('.form-group input');

    // for already filled input
    $inpt.each(function () {
      if ($(this).val() !== '') {
        $(this).addClass('filled')
      }
    });

    //for newly filles input
    $inpt.on('change', function () {
      if ($(this).val() !== '') {
        $(this).addClass('filled');
      } else {
        $(this).removeClass('filled');
      }
    })
  });

  $(document).ready(function () {
    $('.edit-link').click(function () {
      if ($('.form-group--data .form-control').hasClass('edit')) {
        $('.form-group--data .form-control').removeClass('edit');
      } else {
        $('.form-group--data .form-control').addClass('edit');
      }
    });
  });


});




var visibilityToggle = document.querySelector('.visibility');

var input = document.querySelector('.password');

var password = true;

visibilityToggle.addEventListener('click', function () {
  if (password) {
    input.setAttribute('type', 'text');
    visibilityToggle.innerHTML = 'visibility_off';
  } else {
    input.setAttribute('type', 'password');
    visibilityToggle.innerHTML = 'visibility';
  }
  password = !password;

});
